// Проверки криптографии охватывают привязку к полю, изменение шифротекста и соль паролей.
package security

import (
	"encoding/base64"
	"strings"
	"testing"
)

// testVault создаёт независимые фиксированные ключи для проверки криптографических операций.
func testVault(t *testing.T) *Vault {
	t.Helper()
	v, err := NewVault(
		base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 32))),
		base64.StdEncoding.EncodeToString([]byte(strings.Repeat("b", 32))),
	)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

// TestEncryption проверяет случайность nonce, подлинность контекста и независимость ключей поиска.
func TestEncryption(t *testing.T) {
	v := testVault(t)
	first, err := v.Encrypt("Анна", "record:name")
	if err != nil {
		t.Fatal(err)
	}

	second, _ := v.Encrypt("Анна", "record:name")
	if first == second {
		t.Fatal("nonce reused")
	}

	plain, err := v.Decrypt(first, "record:name")
	if err != nil || plain != "Анна" {
		t.Fatal("round trip failed")
	}

	if _, err = v.Decrypt(first, "other:name"); err == nil {
		t.Fatal("AAD not enforced")
	}

	bytes, _ := base64.RawStdEncoding.DecodeString(first[3:])
	bytes[len(bytes)-1] ^= 1
	if _, err = v.Decrypt("v1:"+base64.RawStdEncoding.EncodeToString(bytes), "record:name"); err == nil {
		t.Fatal("tampering accepted")
	}

	other, _ := NewVault(
		base64.StdEncoding.EncodeToString([]byte(strings.Repeat("c", 32))),
		base64.StdEncoding.EncodeToString([]byte(strings.Repeat("b", 32))),
	)
	if _, err = other.Decrypt(first, "record:name"); err == nil {
		t.Fatal("wrong key accepted")
	}

	if v.LoginIndex("Alice") != v.LoginIndex("alice") {
		t.Fatal("case normalization missing")
	}
}

// TestPasswords проверяет уникальность соли и отклонение неправильных паролей и повреждённых хешей.
func TestPasswords(t *testing.T) {
	a, err := HashPassword("длинная парольная фраза")
	if err != nil {
		t.Fatal(err)
	}

	b, _ := HashPassword("длинная парольная фраза")
	if a == b {
		t.Fatal("salt reused")
	}

	if !VerifyPassword(a, "длинная парольная фраза") || VerifyPassword(a, "wrong") ||
		VerifyPassword("$argon2id$invalid", "anything") {
		t.Fatal("password verification failed")
	}
}
