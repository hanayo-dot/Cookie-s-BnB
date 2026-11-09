package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/cookies-bnb/platform/services/api/internal/auth"
	"github.com/cookies-bnb/platform/services/api/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = gin.ReleaseMode
	}
	gin.SetMode(ginMode)

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()

	database, err := pgxpool.New(startupCtx, databaseURL)
	if err != nil {
		log.Fatalf("configure database pool: %v", err)
	}
	defer database.Close()
	if err := database.Ping(startupCtx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	secureCookies := true
	if value := os.Getenv("COOKIE_SECURE"); value != "" {
		secureCookies, err = strconv.ParseBool(value)
		if err != nil {
			log.Fatalf("parse COOKIE_SECURE: %v", err)
		}
	}
	allowedOrigin := os.Getenv("APP_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:3000"
	}

	authStore := auth.NewPostgresStore(database)
	authService := auth.NewService(authStore)
	authHandler := httpapi.NewAuthHandler(authService, secureCookies)

	address := os.Getenv("HTTP_ADDR")
	if address == "" {
		address = ":8080"
	}

	server := &http.Server{
		Addr:              address,
		Handler:           httpapi.NewRouter(database.Ping, authHandler, allowedOrigin),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("API listening on %s", address)
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve API: %v", err)
		}
	case <-serverCtx.Done():
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown API: %v", err)
		}
	}
}
