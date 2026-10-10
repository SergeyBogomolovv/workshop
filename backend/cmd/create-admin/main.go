// Команда create-admin создаёт администратора с интерактивным скрытым вводом пароля.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"workshop/internal/config"
	"workshop/internal/domain"
	"workshop/internal/repository/postgres"
	"workshop/internal/security"
	"workshop/internal/service"
)

// main запрашивает обязательные данные профиля и создаёт привилегированную учётную запись.
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Не удалось создать администратора:", err)
		os.Exit(1)
	}
}

// run настраивает зависимости и запрашивает учётные данные без передачи секретов в командной строке.
func run() error {
	c, err := config.Load()
	if err != nil {
		return err
	}

	vault, err := security.NewVault(c.DataKey, c.LoginKey)
	if err != nil {
		return err
	}

	repo, err := postgres.Open(context.Background(), c.DSN)
	if err != nil {
		return fmt.Errorf("база данных недоступна")
	}

	defer repo.Close()
	app, err := service.New(repo, vault)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)
	r := service.Registration{PolicyVersion: domain.PolicyVersion}
	for _, field := range []struct {
		label string
		value *string
	}{
		{"Логин", &r.Login},
		{"Имя", &r.Name},
		{"Телефон (+79991234567)", &r.Phone},
		{"Email", &r.Email},
	} {
		fmt.Print(field.label + ": ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		*field.value = strings.TrimSpace(line)
	}

	fmt.Printf(
		"Политика обработки данных: %s/privacy (версия %s). Согласны? Напишите да: ",
		c.Origin,
		domain.PolicyVersion,
	)
	consent, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	r.Consent = strings.TrimSpace(consent) == "да"
	fmt.Print("Пароль (15–128 символов): ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return err
	}

	r.Password = string(password)
	if err = app.Register(context.Background(), r, "admin"); err != nil {
		return err
	}

	fmt.Println("Администратор создан.")

	return nil
}
