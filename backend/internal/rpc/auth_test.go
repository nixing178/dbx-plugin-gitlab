package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func passwordConnectionPayload(t *testing.T, server *httptest.Server, password string) json.RawMessage {
	t.Helper()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	port, _ := strconv.Atoi(parsed.Port())

	payload := map[string]any{
		"connection": map[string]any{
			"id":       "c3",
			"name":     "GitLab",
			"host":     parsed.Hostname(),
			"port":     port,
			"username": "demo",
			"secrets":  map[string]any{"password": password},
			"external_config": map[string]any{
				"scheme":      parsed.Scheme,
				"auth_method": "password",
				"read_only":   true,
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return raw
}

func TestPasswordAuthConnection(t *testing.T) {
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			_ = r.ParseForm()
			form = r.PostForm
			_, _ = w.Write([]byte(`{"access_token":"access-1","expires_in":7200,"refresh_token":"refresh-1"}`))
		case "/api/v4/user":
			if r.Header.Get("Authorization") != "Bearer access-1" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message":"401 Unauthorized"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
		case "/api/v4/version":
			_, _ = w.Write([]byte(`{"version":"17.5.5"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	}))
	t.Cleanup(server.Close)

	service := NewService()
	result, rpcErr := service.Handle(
		context.Background(),
		"connection/test",
		passwordConnectionPayload(t, server, "secret"),
	)
	if rpcErr != nil {
		t.Fatalf("connection/test: %v", rpcErr)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if payload["success"] != true || payload["version"] != "17.5.5" {
		t.Fatalf("payload = %#v", payload)
	}
	if form.Get("grant_type") != "password" ||
		form.Get("username") != "demo" ||
		form.Get("password") != "secret" {
		t.Fatalf("form = %v", form)
	}
}

func TestPasswordAuthRejectedCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/oauth/token" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"invalid credentials"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	service := NewService()
	_, rpcErr := service.Handle(
		context.Background(),
		"connection/test",
		passwordConnectionPayload(t, server, "wrong"),
	)
	if rpcErr == nil {
		t.Fatal("expected an error")
	}
	if rpcErr.Kind != "unauthorized" {
		t.Fatalf("kind = %q, message = %q", rpcErr.Kind, rpcErr.Message)
	}
}
