package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	response := httptest.NewRecorder()

	NewRouter(nil, nil, "").ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ok\"}" {
		t.Fatalf("unexpected response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOriginGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(originGuard("https://bnb.example"))
	router.POST("/mutation", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	tests := []struct {
		name       string
		origin     string
		wantStatus int
	}{
		{name: "missing origin", wantStatus: http.StatusForbidden},
		{name: "foreign origin", origin: "https://attacker.example", wantStatus: http.StatusForbidden},
		{name: "allowed origin", origin: "https://bnb.example", wantStatus: http.StatusNoContent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/mutation", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("got status %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}
func TestReadyz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		ping       DatabasePinger
		wantStatus int
		wantBody   string
	}{
		{
			name:       "database reachable",
			ping:       func(context.Context) error { return nil },
			wantStatus: http.StatusOK,
			wantBody:   "{\"status\":\"ready\"}",
		},
		{
			name:       "database unavailable",
			ping:       func(context.Context) error { return errors.New("connection refused") },
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "{\"status\":\"not_ready\"}",
		},
		{
			name:       "database not configured",
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "{\"status\":\"not_ready\"}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil)
			response := httptest.NewRecorder()

			NewRouter(test.ping, nil, "").ServeHTTP(response, request)

			if response.Code != test.wantStatus || response.Body.String() != test.wantBody {
				t.Fatalf("unexpected response: status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
