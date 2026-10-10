// Пакет handler преобразует HTTP-запросы в вызовы сервисов приложения.
package handler

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"workshop/internal/config"
	"workshop/internal/domain"
	mw "workshop/internal/middleware"
	"workshop/internal/service"
)

// Handler связывает сервисы приложения с маршрутами API.
type Handler struct {
	service *service.Service
	config  config.Config
	limiter *mw.Limiter
}

// New создаёт маршрутизатор API с едиными правилами защиты.
func New(s *service.Service, c config.Config) http.Handler {
	h := &Handler{
		s,
		c,
		mw.NewLimiter(),
	}
	r := chi.NewRouter()
	r.Use(mw.Recover)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(mw.Session(s), mw.Origin(c.Origin))
		r.Get("/catalog", h.catalog)
		r.Post("/auth/register", h.register)
		r.Post("/auth/login", h.login)
		r.Get("/auth/session", h.sessionState)
		r.Group(func(r chi.Router) {
			r.Use(mw.Require(""))
			r.Get("/auth/me", h.me)
			r.Get("/auth/csrf", h.csrf)
			r.Post("/auth/logout", h.logout)
			r.Get("/profile", h.me)
			r.Put("/profile", h.profile)
			r.Get("/slots", h.slots)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.Require("client"))
			r.Get("/appointments", h.list)
			r.Post("/appointments", h.book)
			r.Get("/appointments/{id}", h.detail)
			r.Post("/appointments/{id}/cancel", h.cancel)
		})
		r.Route("/admin", func(r chi.Router) {
			r.Use(mw.Require("admin"))
			r.Get("/appointments", h.list)
			r.Get("/appointments/{id}", h.detail)
			r.Patch("/appointments/{id}/status", h.status)
		})
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		mw.Error(w, domain.ErrNotFound)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		mw.Error(w, domain.Fail(405, "method", "Метод не поддерживается"))
	})

	return r
}

// sessionState определяет гостевое состояние без ожидаемой ошибки и ослабления защищённых маршрутов.
func (h *Handler) sessionState(w http.ResponseWriter, r *http.Request) {
	identity, ok := mw.Current(r)
	if !ok {
		mw.JSON(w, 200, map[string]any{"user": nil})

		return
	}

	mw.JSON(w, 200, map[string]any{"user": identity.User})
}

// decode принимает один JSON-объект ограниченного размера и отклоняет неизвестные поля.
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		mw.Error(w, domain.Fail(400, "content_type", "Ожидается JSON"))

		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		mw.Error(w, domain.Fail(400, "json", "Проверьте формат и обязательные поля запроса"))

		return false
	}

	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		mw.Error(w, domain.Fail(400, "json", "Ожидается один JSON-объект"))

		return false
	}

	return true
}

// respond централизованно преобразует результаты сервиса и ошибки в ответы.
func respond(w http.ResponseWriter, data any, err error) {
	if err != nil {
		mw.Error(w, err)

		return
	}

	mw.JSON(w, 200, data)
}

// limited сообщает время повторной попытки при исчерпании лимита аутентификации.
func (h *Handler) limited(w http.ResponseWriter, key string, limit int) bool {
	if h.limiter.Allow(key, limit) {
		return false
	}

	w.Header().Set("Retry-After", "900")
	mw.Error(w, domain.Fail(429, "rate_limit", "Слишком много попыток. Попробуйте через 15 минут."))

	return true
}

// register создаёт клиента только после проверки явного согласия.
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	if h.limited(w, "register:"+mw.IP(r), h.config.RegisterIP) {
		return
	}

	var input service.Registration
	if !decode(w, r, &input) {
		return
	}

	if err := h.service.Register(r.Context(), input, "client"); err != nil {
		mw.Error(w, err)

		return
	}

	mw.JSON(w, 201, map[string]bool{"created": true})
}

// login проверяет лимиты IP и учётной записи до вычисления хеша пароля.
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if h.limited(w, "login-ip:"+mw.IP(r), h.config.LoginIP) {
		return
	}

	var input struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}

	if h.limited(w, "login-account:"+h.service.LoginKey(input.Login), h.config.LoginAccount) {
		return
	}

	token, user, err := h.service.Login(r.Context(), input.Login, input.Password)
	if err != nil {
		mw.Error(w, err)

		return
	}

	http.SetCookie(
		w,
		&http.Cookie{
			Name:     "workshop_session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400,
			Expires:  time.Now().Add(24 * time.Hour),
		},
	)
	mw.JSON(w, 200, user)
}

// me возвращает текущий профиль, не раскрывая служебные поля защиты.
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	identity, _ := mw.Current(r)
	mw.JSON(w, 200, identity.User)
}

// csrf возвращает токен проверки запросов текущей сессии.
func (h *Handler) csrf(w http.ResponseWriter, r *http.Request) {
	identity, _ := mw.Current(r)
	mw.JSON(w, 200, map[string]string{"token": identity.Session.CSRF})
}

// logout отзывает текущий токен до удаления cookie из браузера.
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	identity, _ := mw.Current(r)
	if err := h.service.Logout(r.Context(), identity.Session.ID); err != nil {
		mw.Error(w, err)

		return
	}

	http.SetCookie(
		w,
		&http.Cookie{
			Name:     "workshop_session",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
			Expires:  time.Unix(1, 0),
		},
	)
	w.WriteHeader(204)
}

// profile обновляет проверенные контактные данные текущего пользователя.
func (h *Handler) profile(w http.ResponseWriter, r *http.Request) {
	var input service.Profile
	if !decode(w, r, &input) {
		return
	}

	identity, _ := mw.Current(r)
	user, err := h.service.UpdateProfile(r.Context(), identity.User, input)
	respond(w, user, err)
}

// catalog возвращает общедоступные категории устройств и услуги.
func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Catalog(r.Context())
	respond(w, data, err)
}

// slots возвращает свободные интервалы приёма для проверенной даты.
func (h *Handler) slots(w http.ResponseWriter, r *http.Request) {
	if !query(w, r, "date") {
		return
	}

	data, err := h.service.Slots(r.Context(), r.URL.Query().Get("date"))
	respond(w, data, err)
}

// query отклоняет неизвестные или повторяющиеся параметры запроса.
func query(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		mw.Error(w, domain.Fail(400, "query", "Некорректные параметры запроса"))

		return false
	}

	for key, values := range values {
		found := false
		for _, name := range allowed {
			if key == name {
				found = true
			}
		}

		if !found || len(values) != 1 {
			mw.Error(w, domain.Fail(400, "query", "Некорректные параметры запроса"))

			return false
		}
	}

	return true
}

// list применяет ограничения пагинации и доступа пользователя.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	if !query(w, r, "page", "status") {
		return
	}

	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		var err error
		page, err = strconv.Atoi(raw)
		if err != nil {
			mw.Error(w, domain.Fail(400, "page", "Некорректный номер страницы"))

			return
		}
	}

	identity, _ := mw.Current(r)
	data, err := h.service.List(r.Context(), identity.User, r.URL.Query().Get("status"), page)
	respond(w, data, err)
}

// book создаёт заявку на ремонт, связанную с владельцем.
func (h *Handler) book(w http.ResponseWriter, r *http.Request) {
	var input service.Booking
	if !decode(w, r, &input) {
		return
	}

	identity, _ := mw.Current(r)
	id, err := h.service.Book(r.Context(), identity.User, input)
	if err != nil {
		mw.Error(w, err)

		return
	}

	mw.JSON(w, 201, map[string]string{"id": id})
}

// detail возвращает заявку только после проверки прав пользователя.
func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	identity, _ := mw.Current(r)
	data, err := h.service.Detail(r.Context(), identity.User, chi.URLParam(r, "id"))
	respond(w, data, err)
}

// cancel запрашивает единственный переход статуса, доступный клиенту.
func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "cancelled")
}

// status разбирает запрошенный администратором переход статуса.
func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &input) {
		return
	}

	h.transition(w, r, input.Status)
}

// transition отвечает только после успешного атомарного изменения статуса.
func (h *Handler) transition(w http.ResponseWriter, r *http.Request, to string) {
	identity, _ := mw.Current(r)
	if err := h.service.ChangeStatus(r.Context(), identity.User, chi.URLParam(r, "id"), to); err != nil {
		mw.Error(w, err)

		return
	}

	w.WriteHeader(204)
}
