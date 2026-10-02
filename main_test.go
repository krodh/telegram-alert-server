package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeTelegram stands in for the Bot API and records the last message sent.
func fakeTelegram(t *testing.T, status int) *map[string]any {
	t.Helper()
	got := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottest-token/sendMessage" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("bad payload: %v", err)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)

	oldAPI, oldToken, oldChat := telegramAPI, botToken, chatID
	telegramAPI, botToken, chatID = srv.URL, "test-token", "12345"
	t.Cleanup(func() { telegramAPI, botToken, chatID = oldAPI, oldToken, oldChat })
	return &got
}

func TestExtractText(t *testing.T) {
	cases := map[string]string{
		`{"text":"deploy finished"}`:       "deploy finished",
		`{"message":"  disk full  "}`:      "disk full",
		`{"text":"","message":"fallback"}`: "fallback",
		`{"other":"x"}`:                    `{"other":"x"}`,
		"plain text":                       "plain text",
		"   ":                              "",
	}
	for in, want := range cases {
		if got := extractText([]byte(in)); got != want {
			t.Errorf("extractText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	healthHandler(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	healthHandler(rec, httptest.NewRequest(http.MethodPost, "/health", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /health: got %d", rec.Code)
	}
}

func TestAlertSends(t *testing.T) {
	got := fakeTelegram(t, http.StatusOK)
	rec := httptest.NewRecorder()
	alertHandler(rec, httptest.NewRequest(http.MethodPost, "/alert", strings.NewReader("disk usage above 90%\n")))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if (*got)["text"] != "disk usage above 90%" {
		t.Errorf("text = %v", (*got)["text"])
	}
	if (*got)["chat_id"] != float64(12345) {
		t.Errorf("numeric chat_id should be sent as a number, got %#v", (*got)["chat_id"])
	}
}

func TestWebhookSends(t *testing.T) {
	got := fakeTelegram(t, http.StatusOK)
	rec := httptest.NewRecorder()
	webhookHandler(rec, httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"text":"deploy finished"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if (*got)["text"] != "deploy finished" {
		t.Errorf("text = %v", (*got)["text"])
	}
}

func TestRejects(t *testing.T) {
	fakeTelegram(t, http.StatusOK)
	tests := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		body    string
		want    int
	}{
		{"alert GET", alertHandler, http.MethodGet, "", http.StatusMethodNotAllowed},
		{"alert empty", alertHandler, http.MethodPost, "  ", http.StatusBadRequest},
		{"webhook GET", webhookHandler, http.MethodGet, "", http.StatusMethodNotAllowed},
		{"webhook empty", webhookHandler, http.MethodPost, "", http.StatusBadRequest},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		tt.handler(rec, httptest.NewRequest(tt.method, "/", strings.NewReader(tt.body)))
		if rec.Code != tt.want {
			t.Errorf("%s: got %d, want %d", tt.name, rec.Code, tt.want)
		}
	}
}

func TestTelegramError(t *testing.T) {
	fakeTelegram(t, http.StatusUnauthorized)
	rec := httptest.NewRecorder()
	alertHandler(rec, httptest.NewRequest(http.MethodPost, "/alert", strings.NewReader("hi")))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("got %d, want 502", rec.Code)
	}
}
