// Проверки бизнес-правил охватывают все переходы статусов и граничные даты без инфраструктуры.
package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"workshop/internal/domain"
)

// TestTransitions проверяет все сочетания статусов и ролей.
func TestTransitions(t *testing.T) {
	statuses := []string{
		"new",
		"accepted",
		"in_progress",
		"ready",
		"completed",
		"cancelled",
		"unknown",
	}
	allowed := map[string]bool{
		"new:accepted":         true,
		"accepted:in_progress": true,
		"in_progress:ready":    true,
		"ready:completed":      true,
		"new:cancelled":        true,
		"accepted:cancelled":   true,
	}
	for _, from := range statuses {
		for _, to := range statuses {
			for _, admin := range []bool{false, true} {
				want := allowed[from+":"+to] && (admin || to == "cancelled")
				if CanTransition(from, to, admin) != want {
					t.Fatalf("unexpected transition %s -> %s (admin=%v)", from, to, admin)
				}
			}
		}
	}
}

// TestRegistrationValidation отклоняет недопустимые значения до обращения к репозиторию.
func TestRegistrationValidation(t *testing.T) {
	valid := Registration{
		Login:         "client_name",
		Password:      "long password phrase",
		Name:          "Анна",
		Phone:         "+79991234567",
		Email:         "test@example.ru",
		Consent:       true,
		PolicyVersion: domain.PolicyVersion,
	}
	cases := []func(*Registration){
		func(r *Registration) {
			r.Login = "abc"
		},
		func(r *Registration) {
			r.Login = "1name"
		},
		func(r *Registration) {
			r.Login = strings.Repeat("a", 33)
		},
		func(r *Registration) {
			r.Password = strings.Repeat("я", 14)
		},
		func(r *Registration) {
			r.Password = strings.Repeat("я", 129)
		},
		func(r *Registration) {
			r.Consent = false
		},
		func(r *Registration) {
			r.PolicyVersion = "outdated"
		},
		func(r *Registration) {
			r.Name = ""
		},
		func(r *Registration) {
			r.Phone = ""
		},
		func(r *Registration) {
			r.Email = ""
		},
	}
	s := &Service{}
	for i, change := range cases {
		input := valid
		change(&input)
		if err := s.Register(context.Background(), input, "client"); err == nil {
			t.Fatalf("invalid registration %d accepted", i)
		}
	}
}

// TestBookingWindow проверяет границы по Москве, горизонт в 30 дней и точное время начала.
func TestBookingWindow(t *testing.T) {
	zone, _ := time.LoadLocation("Europe/Moscow")
	now := time.Date(2026, 10, 10, 10, 0, 0, 0, zone)
	s := Service{
		zone: zone,
		now: func() time.Time {
			return now
		},
	}
	cases := []struct {
		slot time.Time
		want bool
	}{
		{now, false},
		{now.Add(time.Hour), true},
		{now.Add(time.Minute), false},
		{time.Date(2026, 10, 10, 18, 0, 0, 0, zone), false},
		{time.Date(2026, 11, 8, 17, 0, 0, 0, zone), true},
		{time.Date(2026, 11, 9, 10, 0, 0, 0, zone), false},
		{time.Date(2026, 10, 11, 10, 0, 0, 1, zone), false},
	}
	for _, test := range cases {
		if s.validSlot(test.slot) != test.want {
			t.Fatalf("slot validation failed: %v", test.slot)
		}
	}
}

// TestProfileValidation отклоняет неверные обязательные контакты и сохраняет имена на Unicode.
func TestProfileValidation(t *testing.T) {
	valid := Profile{
		" Анна-Мария ",
		"+79991234567",
		"person@example.ru",
	}
	if err := ValidateProfile(&valid); err != nil || valid.Name != "Анна-Мария" {
		t.Fatal("valid profile rejected")
	}

	cases := []Profile{
		{
			" ",
			valid.Phone,
			valid.Email,
		},
		{
			"123",
			valid.Phone,
			valid.Email,
		},
		{
			valid.Name,
			"89991234567",
			valid.Email,
		},
		{
			valid.Name,
			valid.Phone,
			"Name <p@example.ru>",
		},
		{
			valid.Name,
			valid.Phone,
			"",
		},
	}
	for _, p := range cases {
		if ValidateProfile(&p) == nil {
			t.Fatal("invalid profile accepted")
		}
	}
}
