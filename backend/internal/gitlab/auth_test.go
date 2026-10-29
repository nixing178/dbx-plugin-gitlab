package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

type authServerState struct {
	tokenCalls  int32
	refreshCall int32
	lastForm    url.Values
	lastAuth    string
	tokenBody   string
	refreshBody string
}

func newAuthServer(t *testing.T, state *authServerState) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/oauth/token" {
			_ = r.ParseForm()
			state.lastForm = r.PostForm
			grant := r.PostForm.Get("grant_type")

			if grant == "refresh_token" {
				atomic.AddInt32(&state.refreshCall, 1)
				if state.refreshBody == "" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
					return
				}
				_, _ = w.Write([]byte(state.refreshBody))
				return
			}

			atomic.AddInt32(&state.tokenCalls, 1)
			if state.tokenBody == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"invalid credentials"}`))
				return
			}
			_, _ = w.Write([]byte(state.tokenBody))
			return
		}

		if r.URL.Path == "/api/v4/user" {
			state.lastAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestPasswordAuthExchangesToken(t *testing.T) {
	state := &authServerState{
		tokenBody: `{"access_token":"access-1","token_type":"bearer","expires_in":7200,"refresh_token":"refresh-1"}`,
	}
	server := newAuthServer(t, state)

	client, err := NewClient(Config{
		BaseURL:  server.URL,
		Username: "demo",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.CloseIdleConnections)

	user, err := client.WhoAmI(context.Background())
	if err != nil {
		t.Fatalf("WhoAmI: %v", err)
	}
	if user.Username != "demo" {
		t.Fatalf("user = %+v", user)
	}
	if atomic.LoadInt32(&state.tokenCalls) != 1 {
		t.Fatalf("token calls = %d", state.tokenCalls)
	}
	if state.lastForm.Get("grant_type") != "password" ||
		state.lastForm.Get("username") != "demo" ||
		state.lastForm.Get("password") != "secret" {
		t.Fatalf("form = %v", state.lastForm)
	}
	if state.lastAuth != "Bearer access-1" {
		t.Fatalf("Authorization = %q", state.lastAuth)
	}
}

func TestPasswordAuthReusesValidToken(t *testing.T) {
	state := &authServerState{
		tokenBody: `{"access_token":"access-1","expires_in":7200,"refresh_token":"refresh-1"}`,
	}
	server := newAuthServer(t, state)

	client, err := NewClient(Config{BaseURL: server.URL, Username: "demo", Password: "secret"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.CloseIdleConnections)

	for index := 0; index < 3; index++ {
		if _, err := client.WhoAmI(context.Background()); err != nil {
			t.Fatalf("WhoAmI #%d: %v", index, err)
		}
	}
	if atomic.LoadInt32(&state.tokenCalls) != 1 {
		t.Fatalf("token must be exchanged once, got %d", state.tokenCalls)
	}
	if atomic.LoadInt32(&state.refreshCall) != 0 {
		t.Fatalf("refresh must not be called for a valid token, got %d", state.refreshCall)
	}
}

func TestPasswordAuthRefreshesExpiringToken(t *testing.T) {
	state := &authServerState{
		// expires_in=60 会立即进入提前刷新窗口。
		tokenBody:   `{"access_token":"access-1","expires_in":60,"refresh_token":"refresh-1"}`,
		refreshBody: `{"access_token":"access-2","expires_in":7200,"refresh_token":"refresh-2"}`,
	}
	server := newAuthServer(t, state)

	client, err := NewClient(Config{BaseURL: server.URL, Username: "demo", Password: "secret"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.CloseIdleConnections)

	if _, err := client.WhoAmI(context.Background()); err != nil {
		t.Fatalf("first WhoAmI: %v", err)
	}
	if _, err := client.WhoAmI(context.Background()); err != nil {
		t.Fatalf("second WhoAmI: %v", err)
	}

	if atomic.LoadInt32(&state.refreshCall) != 1 {
		t.Fatalf("refresh calls = %d", state.refreshCall)
	}
	if state.lastForm.Get("grant_type") != "refresh_token" ||
		state.lastForm.Get("refresh_token") != "refresh-1" {
		t.Fatalf("refresh form = %v", state.lastForm)
	}
	if state.lastAuth != "Bearer access-2" {
		t.Fatalf("Authorization = %q", state.lastAuth)
	}
}

func TestPasswordAuthFallsBackWhenRefreshFails(t *testing.T) {
	state := &authServerState{
		tokenBody:   `{"access_token":"access-1","expires_in":60,"refresh_token":"refresh-1"}`,
		refreshBody: "", // 返回 400
	}
	server := newAuthServer(t, state)

	client, err := NewClient(Config{BaseURL: server.URL, Username: "demo", Password: "secret"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.CloseIdleConnections)

	if _, err := client.WhoAmI(context.Background()); err != nil {
		t.Fatalf("first WhoAmI: %v", err)
	}
	if _, err := client.WhoAmI(context.Background()); err != nil {
		t.Fatalf("second WhoAmI after failed refresh: %v", err)
	}
	if atomic.LoadInt32(&state.tokenCalls) != 2 {
		t.Fatalf("password grant must be retried after a failed refresh, got %d", state.tokenCalls)
	}
}

func TestPasswordAuthInvalidCredentials(t *testing.T) {
	state := &authServerState{tokenBody: ""}
	server := newAuthServer(t, state)

	client, err := NewClient(Config{BaseURL: server.URL, Username: "demo", Password: "wrong"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.CloseIdleConnections)

	_, err = client.WhoAmI(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Kind != ErrUnauthorized {
		t.Fatalf("kind = %q", apiErr.Kind)
	}
}

func TestNewClientRequiresCredentials(t *testing.T) {
	if _, err := NewClient(Config{BaseURL: "https://gitlab.example.com"}); err == nil {
		t.Fatal("expected an error when no credentials are provided")
	}
	if _, err := NewClient(Config{BaseURL: "https://gitlab.example.com", Username: "demo"}); err == nil {
		t.Fatal("expected an error when the password is missing")
	}
}

func TestPasswordAuthSendsClientCredentialsWhenConfigured(t *testing.T) {
	state := &authServerState{
		tokenBody: `{"access_token":"access-1","expires_in":7200}`,
	}
	server := newAuthServer(t, state)

	client, err := NewClient(Config{
		BaseURL:      server.URL,
		Username:     "demo",
		Password:     "secret",
		ClientID:     "app-id",
		ClientSecret: "app-secret",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.CloseIdleConnections)

	if _, err := client.WhoAmI(context.Background()); err != nil {
		t.Fatalf("WhoAmI: %v", err)
	}
	if state.lastForm.Get("client_id") != "app-id" || state.lastForm.Get("client_secret") != "app-secret" {
		t.Fatalf("form = %v", state.lastForm)
	}
}
