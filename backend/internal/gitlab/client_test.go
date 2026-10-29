package gitlab

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	client, err := NewClient(Config{BaseURL: server.URL, Token: "test-token"})
	if err != nil {
		server.Close()
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() {
		client.CloseIdleConnections()
		server.Close()
	})
	return client
}

func TestClientSendsAuthTokenAndUserAgent(t *testing.T) {
	var (
		token string
		agent string
		path  string
	)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		token = r.Header.Get("PRIVATE-TOKEN")
		agent = r.Header.Get("User-Agent")
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
	})

	user, err := client.WhoAmI(context.Background())
	if err != nil {
		t.Fatalf("WhoAmI: %v", err)
	}
	if user.Username != "demo" {
		t.Fatalf("username = %q", user.Username)
	}
	if token != "test-token" {
		t.Fatalf("PRIVATE-TOKEN = %q", token)
	}
	if !strings.HasPrefix(agent, "dbx-gitlab-plugin/0.2.4") {
		t.Fatalf("User-Agent = %q", agent)
	}
	if path != "/api/v4/user" {
		t.Fatalf("path = %q", path)
	}
}

func TestClientMapsHTTPStatusToKinds(t *testing.T) {
	tests := []struct {
		status int
		kind   string
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusForbidden, ErrForbidden},
		{http.StatusNotFound, ErrNotFound},
		{http.StatusConflict, ErrConflict},
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusBadGateway, ErrServer},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"message":"boom"}`))
			})

			_, err := client.WhoAmI(context.Background())
			if err == nil {
				t.Fatal("expected an error")
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError, got %T", err)
			}
			if apiErr.Kind != tt.kind {
				t.Fatalf("kind = %q, want %q", apiErr.Kind, tt.kind)
			}
			if apiErr.Status != tt.status {
				t.Fatalf("status = %d, want %d", apiErr.Status, tt.status)
			}
			if apiErr.Message != "boom" {
				t.Fatalf("message = %q", apiErr.Message)
			}
		})
	}
}

func TestClientParsesValidationMessageArray(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":["title is too long","description is required"]}`))
	})

	_, err := client.WhoAmI(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %v", err)
	}
	if apiErr.Message != "title is too long; description is required" {
		t.Fatalf("message = %q", apiErr.Message)
	}
}

func TestClientReadsRetryAfter(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "42")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := client.WhoAmI(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %v", err)
	}
	if apiErr.RetryAfter != 42 {
		t.Fatalf("retry after = %d, want 42", apiErr.RetryAfter)
	}
}

func TestNewClientRejectsEmptyToken(t *testing.T) {
	_, err := NewClient(Config{BaseURL: "https://gitlab.example.com"})
	if err == nil {
		t.Fatal("expected an error for missing token")
	}
}

// newTestServerForConfig 允许测试覆盖 Config 的其它字段（如 MaxFileBytes）。
func newTestServerForConfig(t *testing.T, cfg Config, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	cfg.BaseURL = server.URL
	if cfg.Token == "" {
		cfg.Token = "test-token"
	}
	client, err := NewClient(cfg)
	if err != nil {
		server.Close()
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() {
		client.CloseIdleConnections()
		server.Close()
	})
	return client
}
