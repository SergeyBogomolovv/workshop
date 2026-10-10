// Команда api запускает HTTP-сервис и обеспечивает корректное завершение работы.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"workshop/internal/config"
	"workshop/internal/handler"
	"workshop/internal/repository/postgres"
	"workshop/internal/security"
	"workshop/internal/service"
)

// main связывает инфраструктуру и слои приложения без журналирования секретов.
func main() {
	c, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	vault, err := security.NewVault(c.DataKey, c.LoginKey)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	repo, err := postgres.Open(ctx, c.DSN)
	cancel()
	if err != nil {
		log.Fatal("database initialization failed")
	}

	defer repo.Close()
	app, err := service.New(repo, vault)
	if err != nil {
		log.Fatal("service initialization failed")
	}

	server := &http.Server{
		Addr:              c.Address,
		Handler:           handler.New(app, c),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}
	stop, stopCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopCancel()
	go func() {
		<-stop.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	log.Print("API started")
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("HTTP server failed")
	}
}
