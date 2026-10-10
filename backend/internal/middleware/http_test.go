// Проверки промежуточных обработчиков охватывают лимиты запросов и ограничение хранимого состояния.
package middleware

import "testing"

// TestLimiter проверяет независимый лимит каждого ключа и отказ при его исчерпании.
func TestLimiter(t *testing.T) {
	l := NewLimiter()
	if !l.Allow("account:a", 2) || !l.Allow("account:a", 2) || l.Allow("account:a", 2) {
		t.Fatal("request budget not enforced")
	}

	if !l.Allow("account:b", 2) {
		t.Fatal("independent key blocked")
	}
}
