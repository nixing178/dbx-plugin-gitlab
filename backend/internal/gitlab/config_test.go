package gitlab

import "testing"

func TestParseBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		scheme   string
		host     string
		port     int
		basePath string
		want     string
		wantErr  bool
	}{
		{"https with default port", "https", "gitlab.example.com", 443, "", "https://gitlab.example.com", false},
		{"http with custom port and context path", "http", "192.0.2.10", 8080, "gitlab", "http://192.0.2.10:8080/gitlab", false},
		{"https with custom port and slash path", "https", "gitlab.example.com", 8443, "/", "https://gitlab.example.com:8443", false},
		{"http with default port", "http", "gitlab.local", 80, "", "http://gitlab.local", false},
		{"trim spaces and trailing slash", "https", "  gitlab.example.com  ", 443, "/sub/", "https://gitlab.example.com/sub", false},
		{"zero port falls back to scheme default", "https", "gitlab.example.com", 0, "", "https://gitlab.example.com", false},
		{"missing scheme", "", "gitlab.local", 443, "", "", true},
		{"unsupported scheme", "ftp", "gitlab.local", 443, "", "", true},
		{"missing host", "https", "   ", 443, "", "", true},
		{"host contains scheme", "https", "https://gitlab.local", 443, "", "", true},
		{"host contains slash", "https", "gitlab.local/group", 443, "", "", true},
		{"port out of range", "https", "gitlab.local", 70000, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBaseURL(tt.scheme, tt.host, tt.port, tt.basePath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
