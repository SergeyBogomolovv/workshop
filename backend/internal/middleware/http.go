// Пакет middleware проверяет сессии, источник запросов, CSRF и ограничения частоты обращений.
package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"workshop/internal/domain"
	"workshop/internal/service"
)

// Identity содержит проверенные сервером данные учётной записи и сессии.
type Identity struct {
	User    domain.User
	Session domain.Session
}
type identityKey struct{}

// Current извлекает проверенные данные пользователя из запроса, если они есть.
func Current(r *http.Request) (Identity, bool) {
	v, ok := r.Context().Value(identityKey{}).(Identity)

	return v, ok
}

// JSON записывает JSON-ответ с заданным статусом и типом содержимого.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// Error скрывает внутренние ошибки и сохраняет разрешённые сообщения для клиента.
func Error(w http.ResponseWriter, err error) {
	var public *domain.Error
	if !errors.As(err, &public) {
		public = &domain.Error{
			Status:  500,
			Code:    "internal",
			Message: "Не удалось выполнить запрос. Попробуйте позже.",
		}
	}

	JSON(w, public.Status, map[string]string{"code": public.Code, "message": public.Message})
}

// Session проверяет переданный токен на каждом API-запросе и допускает гостей.
func Session(s *service.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			if cookie, err := r.Cookie("workshop_session"); err == nil {
				session, user, err := s.Authenticate(r.Context(), cookie.Value)
				if err == nil {
					r = r.WithContext(
						context.WithValue(r.Context(), identityKey{}, Identity{user, session}),
					)
				} else if !errors.Is(err, domain.ErrNotFound) {
					Error(w, err)

					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Origin требует настроенный источник для каждого изменяющего запроса.
func Origin(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" &&
				r.Header.Get("Origin") != origin {
				Error(w, domain.Fail(403, "origin", "Источник запроса не разрешён"))

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Require проверяет роль пользователя и привязанный к сессии CSRF-токен при изменениях.
func Require(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := Current(r)
			if !ok {
				Error(w, domain.Fail(401, "unauthorized", "Войдите в аккаунт"))

				return
			}

			if role != "" && identity.User.Role != role {
				Error(w, domain.Fail(403, "forbidden", "Недостаточно прав"))

				return
			}

			if r.Method != "GET" && r.Method != "HEAD" &&
				subtle.ConstantTimeCompare(
					[]byte(r.Header.Get("X-CSRF-Token")),
					[]byte(identity.Session.CSRF),
				) != 1 {
				Error(w, domain.Fail(403, "csrf", "Обновите страницу и повторите действие"))

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Recover преобразует неожиданные паники в безопасные ответы без записи пользовательского ввода в журнал.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				Error(w, errors.New("request panic"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// IP читает адрес, установленный единственным доверенным обратным прокси в закрытой сети.
func IP(r *http.Request) string {
	if ip := net.ParseIP(r.Header.Get("X-Real-IP")); ip != nil {
		return ip.String()
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

type entry struct {
	count int
	until time.Time
}

// Limiter ограничивает частоту запросов в фиксированном окне для одного экземпляра API.
type Limiter struct {
	mu        sync.Mutex
	entries   map[string]entry
	nextSweep time.Time
}

// NewLimiter настраивает учёт запросов.
func NewLimiter() *Limiter {
	return &Limiter{entries: make(map[string]entry)}
}

// Allow учитывает запрос и периодически удаляет записи с истёкшим сроком.
func (l *Limiter) Allow(key string, limit int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.After(l.nextSweep) {
		for key, item := range l.entries {
			if !item.until.After(now) {
				delete(l.entries, key)
			}
		}

		l.nextSweep = now.Add(time.Minute)
	}

	item, ok := l.entries[key]
	if !ok || !item.until.After(now) {
		if !ok && len(l.entries) >= 10000 {
			return false
		}

		item = entry{until: now.Add(15 * time.Minute)}
	}

	item.count++
	l.entries[key] = item

	return item.count <= limit
}
