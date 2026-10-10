// Интеграционные тесты проверяют защиту HTTP и реальные транзакции и роли PostgreSQL.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"workshop/internal/config"
	"workshop/internal/domain"
	"workshop/internal/handler"
	"workshop/internal/repository/postgres"
	"workshop/internal/security"
	"workshop/internal/service"
)

// reply собирает статус, заголовки и данные ответа без журналирования учётных данных.
type reply struct {
	status int
	header http.Header
	body   []byte
}

// request выполняет операцию API с явной передачей cookie для HTTP-тестов.
func request(
	t *testing.T,
	client *http.Client,
	base, origin, method, path, cookie, csrf string,
	body any,
) reply {
	t.Helper()
	var data io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}

		data = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, base+"/api/v1"+path, data)
	if err != nil {
		t.Fatal(err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}

	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	return reply{
		res.StatusCode,
		res.Header,
		raw,
	}
}

// expect проверяет статус и ограничивает вывод ошибки безопасными сообщениями API.
func expect(t *testing.T, r reply, status int) {
	t.Helper()
	if r.status != status {
		t.Fatalf("expected HTTP %d, got %d: %s", status, r.status, r.body)
	}
}

// decode читает типизированные ответы тестов независимо от порядка полей.
func decode[T any](t *testing.T, r reply) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(r.body, &v); err != nil {
		t.Fatal(err)
	}

	return v
}

// TestWorkflow проверяет полный сценарий работы ролей на настоящей базе данных.
func TestWorkflow(t *testing.T) {
	if os.Getenv("TEST_DATABASE") != "1" {
		t.Skip("set TEST_DATABASE=1 with the test Compose profile")
	}

	ctx := context.Background()
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	c.LoginIP = 500
	c.LoginAccount = 500
	c.RegisterIP = 500
	vault, err := security.NewVault(c.DataKey, c.LoginKey)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := postgres.Open(ctx, c.DSN)
	if err != nil {
		t.Fatal("runtime DB connection failed")
	}

	defer repo.Close()
	app, err := service.New(repo, vault)
	if err != nil {
		t.Fatal(err)
	}

	adminURL, _ := url.Parse(c.DSN)
	adminURL.User = url.UserPassword("admin", os.Getenv("DB_ADMIN_PASSWORD"))
	db, err := sqlx.Connect("pgx", adminURL.String())
	if err != nil {
		t.Fatal("migration DB connection failed")
	}

	defer db.Close()
	prefix := "test" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	password, _ := security.RandomToken()
	names := []string{
		prefix + "a",
		prefix + "b",
		prefix + "admin",
		prefix + "race",
	}
	defer func() {
		for _, name := range names {
			var id string
			if db.Get(
				&id,
				"SELECT id FROM users WHERE login_index=$1",
				vault.LoginIndex(name),
			) == nil {
				_, _ = db.Exec(
					`DELETE FROM appointment_status_history
					 WHERE actor_id = $1 OR appointment_id IN (
					     SELECT id FROM appointments WHERE user_id = $1
					 )`,
					id,
				)
				_, _ = db.Exec("DELETE FROM appointments WHERE user_id=$1", id)
				_, _ = db.Exec("DELETE FROM sessions WHERE user_id=$1", id)
				_, _ = db.Exec("DELETE FROM users WHERE id=$1", id)
			}
		}
	}()
	registration := func(login string) service.Registration {
		return service.Registration{
			Login:         login,
			Password:      password,
			Name:          "Тестовый Клиент",
			Phone:         "+79991234567",
			Email:         "test@example.ru",
			Consent:       true,
			PolicyVersion: domain.PolicyVersion,
		}
	}
	baseURL := os.Getenv("TEST_BASE_URL")
	throughProxy := baseURL != ""
	if !throughProxy {
		server := httptest.NewServer(handler.New(app, c))
		defer server.Close()
		baseURL = server.URL
	}

	client := &http.Client{Timeout: 15 * time.Second}
	call := func(method, path, cookie, csrf string, body any) reply {
		return request(t, client, baseURL, c.Origin, method, path, cookie, csrf, body)
	}
	t.Run("public_and_validation", func(t *testing.T) {
		expect(t, call("GET", "/auth/session", "", "", nil), 200)
		catalogResponse := call("GET", "/catalog", "", "", nil)
		expect(t, catalogResponse, 200)
		if throughProxy {
			if catalogResponse.header.Get("X-Content-Type-Options") != "nosniff" ||
				!strings.Contains(
					catalogResponse.header.Get("Content-Security-Policy"),
					"script-src 'self'",
				) {
				t.Fatal("missing proxy security headers")
			}
		}
		expect(t, call("GET", "/appointments", "", "", nil), 401)
		bad := registration(names[0])
		bad.Consent = false
		expect(t, call("POST", "/auth/register", "", "", bad), 400)
		expect(t, call("POST", "/auth/register", "", "", map[string]any{"role": "admin"}), 400)
		expect(
			t,
			request(
				t,
				client,
				baseURL,
				"http://untrusted.invalid",
				"POST",
				"/auth/login",
				"",
				"",
				map[string]string{"login": names[0], "password": password},
			),
			403,
		)
	})
	for _, name := range names[:2] {
		expect(t, call("POST", "/auth/register", "", "", registration(name)), 201)
	}

	if err := app.Register(ctx, registration(names[2]), "admin"); err != nil {
		t.Fatal(err)
	}

	t.Run("login_uniqueness", func(t *testing.T) {
		expect(
			t,
			call("POST", "/auth/register", "", "", registration(strings.ToUpper(names[0]))),
			409,
		)
		var wg sync.WaitGroup
		statuses := make(chan int, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				statuses <- call("POST", "/auth/register", "", "", registration(names[3])).status
			}()
		}

		wg.Wait()
		close(statuses)
		counts := map[int]int{}
		for status := range statuses {
			counts[status]++
		}

		if counts[201] != 1 || counts[409] != 1 {
			t.Fatalf("concurrent login result: %v", counts)
		}
	})
	t.Run("credential_errors", func(t *testing.T) {
		for _, login := range []string{
			names[0],
			"missing_account",
			"x' OR 1=1 --",
		} {
			r := call(
				"POST",
				"/auth/login",
				"",
				"",
				map[string]string{"login": login, "password": "invalid"},
			)
			expect(t, r, 401)
			if decode[map[string]string](t, r)["message"] != "Логин или пароль некорректен" {
				t.Fatal("wrong login error")
			}
		}
	})
	login := func(name string) (string, string) {
		r := call(
			"POST",
			"/auth/login",
			"",
			"",
			map[string]string{"login": name, "password": password},
		)
		expect(t, r, 200)
		cookie := r.header.Get("Set-Cookie")
		if !strings.HasPrefix(cookie, "workshop_session=") || strings.Contains(cookie, "; Secure") {
			t.Fatal("session cookie must support local HTTP")
		}

		for _, attr := range []string{
			"HttpOnly",
			"SameSite=Lax",
			"Path=/",
		} {
			if !strings.Contains(cookie, attr) {
				t.Fatalf("missing cookie attribute %s", attr)
			}
		}

		cookie = strings.Split(cookie, ";")[0]
		csrf := decode[map[string]string](t, call("GET", "/auth/csrf", cookie, "", nil))["token"]

		return cookie, csrf
	}
	cookie, csrf := login(names[0])
	otherCookie, _ := login(names[1])
	adminCookie, adminCSRF := login(names[2])
	user, _ := repo.UserByLogin(ctx, vault.LoginIndex(names[0]))
	zone, _ := time.LoadLocation("Europe/Moscow")
	date := time.Now().In(zone).AddDate(0, 0, 28).Format("2006-01-02")
	available := call("GET", "/slots?date="+date, cookie, "", nil)
	expect(t, available, 200)
	slots := decode[[]time.Time](t, available)
	if len(slots) < 2 {
		t.Fatal("test needs two free future slots")
	}

	booking := service.Booking{
		ServiceID:   "10000000-0000-4000-8000-000000000001",
		Category:    "laptop",
		SlotStart:   slots[0],
		Description: "Тестовая неисправность устройства",
	}
	expect(t, call("POST", "/appointments", cookie, "", booking), 403)
	expect(t, call("GET", "/admin/appointments", cookie, "", nil), 403)
	var id string
	t.Run("concurrent_booking", func(t *testing.T) {
		var wg sync.WaitGroup
		replies := make(chan reply, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				replies <- call("POST", "/appointments", cookie, csrf, booking)
			}()
		}

		wg.Wait()
		close(replies)
		counts := map[int]int{}
		for r := range replies {
			counts[r.status]++
			if r.status == 201 {
				id = decode[map[string]string](t, r)["id"]
			}
		}

		if counts[201] != 1 || counts[409] != 1 {
			t.Fatalf("concurrent booking result: %v", counts)
		}
	})
	if id == "" {
		t.Fatal("booking unavailable")
	}

	t.Run("ownership_and_atomic_history", func(t *testing.T) {
		expect(t, call("GET", "/appointments/"+id, otherCookie, "", nil), 404)
		otherCSRF := decode[map[string]string](t, call("GET", "/auth/csrf", otherCookie, "", nil))["token"]
		expect(t, call("POST", "/appointments/"+id+"/cancel", otherCookie, otherCSRF, nil), 404)
		expect(t, call("GET", "/appointments/not-a-uuid", cookie, "", nil), 400)
		expect(t, call("GET", "/appointments?page=%zz", cookie, "", nil), 400)
		oversized := booking
		oversized.Description = strings.Repeat("x", 20000)
		oversizedStatus := http.StatusBadRequest
		if throughProxy {
			oversizedStatus = http.StatusRequestEntityTooLarge
		}

		expect(t, call("POST", "/appointments", cookie, csrf, oversized), oversizedStatus)
		detail := decode[domain.Detail](t, call("GET", "/appointments/"+id, cookie, "", nil))
		if detail.Appointment.Description != booking.Description || len(detail.History) != 1 ||
			detail.Client != nil {
			t.Fatal("unexpected owner detail")
		}

		if err := repo.Transition(ctx, id, "new", "accepted", uuid.NewString()); err == nil {
			t.Fatal("invalid history actor accepted")
		}

		a, _ := repo.Appointment(ctx, id)
		if a.Status != "new" {
			t.Fatal("failed transaction not rolled back")
		}
	})
	t.Run("administrator_workflow", func(t *testing.T) {
		expect(
			t,
			call(
				"PATCH",
				"/admin/appointments/"+id+"/status",
				adminCookie,
				adminCSRF,
				map[string]string{"status": "ready"},
			),
			409,
		)
		for _, status := range []string{
			"accepted",
			"in_progress",
			"ready",
			"completed",
		} {
			expect(
				t,
				call(
					"PATCH",
					"/admin/appointments/"+id+"/status",
					adminCookie,
					adminCSRF,
					map[string]string{"status": status},
				),
				204,
			)
		}

		expect(t, call("POST", "/appointments/"+id+"/cancel", cookie, csrf, nil), 409)
		d := decode[domain.Detail](t, call("GET", "/appointments/"+id, cookie, "", nil))
		if d.Appointment.Status != "completed" || len(d.History) != 5 {
			t.Fatal("workflow history incomplete")
		}

		adminDetail := decode[domain.Detail](
			t,
			call("GET", "/admin/appointments/"+id, adminCookie, "", nil),
		)
		if adminDetail.Client == nil || adminDetail.Client.Name != "Тестовый Клиент" {
			t.Fatal("admin contacts unavailable")
		}
	})
	t.Run("cancellation_releases_slot", func(t *testing.T) {
		booking.SlotStart = slots[1]
		created := call("POST", "/appointments", cookie, csrf, booking)
		expect(t, created, 201)
		cancelID := decode[map[string]string](t, created)["id"]
		expect(t, call("POST", "/appointments/"+cancelID+"/cancel", cookie, csrf, nil), 204)
		expect(t, call("POST", "/appointments", cookie, csrf, booking), 201)
	})
	t.Run("encryption_and_roles", func(t *testing.T) {
		if strings.Contains(user.Name, "Тестовый") || strings.Contains(user.Email, "example") ||
			!strings.HasPrefix(user.PasswordHash, "$argon2id$") {
			t.Fatal("plaintext stored")
		}

		runtime, err := sqlx.Connect("pgx", c.DSN)
		if err != nil {
			t.Fatal("runtime connect failed")
		}

		defer runtime.Close()
		for _, query := range []string{
			"CREATE TABLE forbidden_test(id int)",
			"CREATE ROLE forbidden_test",
			"UPDATE services SET name='bad' WHERE false",
			"DELETE FROM appointment_status_history WHERE false",
		} {
			if _, err = runtime.Exec(query); err == nil {
				t.Fatalf("forbidden query permitted: %s", query)
			}
		}

		var super, createdb, createrole bool
		const permissionsQuery = `
			SELECT rolsuper, rolcreatedb, rolcreaterole
			FROM pg_roles
			WHERE rolname = 'admin'
		`
		err = db.QueryRow(permissionsQuery).Scan(&super, &createdb, &createrole)
		if err != nil || super || createdb || createrole {
			t.Fatal("admin overprivileged")
		}

		var count int
		if err = runtime.Get(&count, "SELECT count(*) FROM services"); err != nil || count != 6 {
			t.Fatal("catalog read denied")
		}
	})
	t.Run("session_expiry_and_revocation", func(t *testing.T) {
		expect(
			t,
			call("GET", "/auth/me", "workshop_session="+strings.Repeat("x", 43), "", nil),
			401,
		)
		expect(t, call("POST", "/auth/logout", cookie, csrf, nil), 204)
		expect(t, call("GET", "/auth/me", cookie, "", nil), 401)
		cookie, _ = login(names[0])
		_, err := db.Exec(
			"UPDATE sessions SET expires_at=now()-interval '1 second' WHERE user_id=$1",
			user.ID,
		)
		if err != nil {
			t.Fatal(err)
		}

		expect(t, call("GET", "/profile", cookie, "", nil), 401)
	})
	t.Run("browser_cookie_session", func(t *testing.T) {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatal(err)
		}

		browser := &http.Client{Jar: jar, Timeout: 15 * time.Second}
		browserCall := func(method, path, csrf string, body any) reply {
			return request(t, browser, baseURL, c.Origin, method, path, "", csrf, body)
		}

		expect(t, browserCall("POST", "/auth/login", "", map[string]string{
			"login":    names[0],
			"password": password,
		}), 200)
		expect(t, browserCall("GET", "/auth/me", "", nil), 200)
		csrfResponse := browserCall("GET", "/auth/csrf", "", nil)
		expect(t, csrfResponse, 200)
		token := decode[map[string]string](t, csrfResponse)["token"]
		expect(t, browserCall("POST", "/auth/logout", token, nil), 204)
		expect(t, browserCall("GET", "/auth/me", "", nil), 401)
	})
	t.Log(
		fmt.Sprintf(
			"Verified workflow, isolation, crypto storage and role permissions (%d generated users cleaned up)",
			len(names),
		),
	)
}
