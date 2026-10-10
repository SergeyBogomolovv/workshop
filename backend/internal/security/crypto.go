// Пакет security реализует шифрование с проверкой подлинности, хеширование токенов и защиту паролей.
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Vault хранит независимые ключи шифрования и поиска логинов.
type Vault struct {
	aead   cipher.AEAD
	lookup []byte
}

// NewVault проверяет ключи и настраивает шифрование с проверкой подлинности.
func NewVault(dataKey, lookupKey string) (*Vault, error) {
	data, err := base64.StdEncoding.DecodeString(dataKey)
	if err != nil || len(data) != 32 {
		return nil, errors.New("DATA_KEY must encode 32 bytes")
	}

	lookup, err := base64.StdEncoding.DecodeString(lookupKey)
	if err != nil || len(lookup) != 32 {
		return nil, errors.New("LOGIN_KEY must encode 32 bytes")
	}

	if subtle.ConstantTimeCompare(data, lookup) == 1 {
		return nil, errors.New("encryption and lookup keys must differ")
	}

	block, err := aes.NewCipher(data)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &Vault{aead, lookup}, nil
}

// Encrypt связывает значение с записью и полем, используя новый случайный nonce.
func (v *Vault) Encrypt(value, aad string) (string, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	result := v.aead.Seal(nonce, nonce, []byte(value), []byte(aad))

	return "v1:" + base64.RawStdEncoding.EncodeToString(result), nil
}

// Decrypt отклоняет изменённые данные, неизвестные версии и неверный контекст записи.
func (v *Vault) Decrypt(value, aad string) (string, error) {
	if !strings.HasPrefix(value, "v1:") {
		return "", errors.New("unsupported ciphertext")
	}

	raw, err := base64.RawStdEncoding.DecodeString(value[3:])
	if err != nil || len(raw) < v.aead.NonceSize() {
		return "", errors.New("invalid ciphertext")
	}

	plain, err := v.aead.Open(nil, raw[:v.aead.NonceSize()], raw[v.aead.NonceSize():], []byte(aad))

	return string(plain), err
}

// LoginIndex позволяет искать логин без учёта регистра, не сохраняя его открытым текстом.
func (v *Vault) LoginIndex(login string) string {
	mac := hmac.New(sha256.New, v.lookup)
	mac.Write([]byte(strings.ToLower(login)))

	return hex.EncodeToString(mac.Sum(nil))
}

// RandomToken возвращает криптографически случайный непрозрачный токен длиной 256 бит.
func RandomToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)

	return base64.RawURLEncoding.EncodeToString(b), err
}

// TokenHash создаёт необратимое представление токена сессии для хранения в БД.
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}

// HashPassword вычисляет хеш Argon2id с уникальной солью и сохраняет параметры в его представлении.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, 2, 19*1024, 1, 32)

	return fmt.Sprintf(
		"$argon2id$v=19$m=19456,t=2,p=1$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword проверяет поддерживаемый формат хеша и сравнивает результат за постоянное время.
func VerifyPassword(encoded, password string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 6 || p[1] != "argon2id" || p[2] != "v=19" || p[3] != "m=19456,t=2,p=1" {
		return false
	}

	salt, e1 := base64.RawStdEncoding.DecodeString(p[4])
	expected, e2 := base64.RawStdEncoding.DecodeString(p[5])
	if e1 != nil || e2 != nil || len(salt) != 16 || len(expected) != 32 {
		return false
	}

	actual := argon2.IDKey([]byte(password), salt, 2, 19*1024, 1, 32)

	return subtle.ConstantTimeCompare(actual, expected) == 1
}
