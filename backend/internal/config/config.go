// Пакет config читает настройки окружения, не раскрывая учётные данные.
package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config содержит настройки сервера и входные секреты.
type Config struct {
	DSN          string
	Origin       string
	DataKey      string
	LoginKey     string
	Address      string
	LoginIP      int
	LoginAccount int
	RegisterIP   int
}

// Load требует явного указания секретов и HTTP-адреса локального приложения.
func Load() (Config, error) {
	settings := Config{
		Origin:   os.Getenv("APP_ORIGIN"),
		DataKey:  os.Getenv("DATA_KEY"),
		LoginKey: os.Getenv("LOGIN_KEY"),
		Address:  ":8080",
	}

	requiredSettings := []string{
		"DB_PASSWORD",
		"DB_NAME",
		"DATA_KEY",
		"LOGIN_KEY",
		"APP_ORIGIN",
	}
	for _, name := range requiredSettings {
		if os.Getenv(name) == "" {
			return settings, errors.New("missing required setting: " + name)
		}
	}

	origin, err := url.Parse(settings.Origin)
	if err != nil ||
		origin.Scheme != "http" ||
		origin.Host == "" ||
		origin.User != nil ||
		origin.Path != "" ||
		origin.RawQuery != "" ||
		origin.Fragment != "" {
		return settings, errors.New("APP_ORIGIN must be an HTTP origin")
	}

	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}

	if !strings.Contains(host, ":") {
		host += ":5432"
	}

	dbURL := url.URL{
		Scheme:   "postgres",
		Host:     host,
		Path:     "/" + os.Getenv("DB_NAME"),
		User:     url.UserPassword("user", os.Getenv("DB_PASSWORD")),
		RawQuery: "sslmode=disable",
	}
	settings.DSN = dbURL.String()
	if address := os.Getenv("HTTP_ADDRESS"); address != "" {
		settings.Address = address
	}

	settings.LoginIP, err = limit("LOGIN_IP_LIMIT", 30)
	if err != nil {
		return settings, err
	}

	settings.LoginAccount, err = limit("LOGIN_ACCOUNT_LIMIT", 10)
	if err != nil {
		return settings, err
	}

	settings.RegisterIP, err = limit("REGISTER_IP_LIMIT", 10)

	return settings, err
}

// limit проверяет, что ограничение частоты запросов положительно и не превышает допустимый предел.
func limit(name string, fallback int) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 10000 {
		return 0, errors.New("invalid limit: " + name)
	}

	return n, nil
}
