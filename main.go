// telegram-alert-server: a minimal HTTP API that relays alerts to a Telegram
// chat via the Bot API. Credentials come from the environment.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	maxBodyBytes    = 1 << 20 // 1MB
	telegramTimeout = 10 * time.Second
)

var (
	botToken string
	chatID   string

	// telegramAPI is the Bot API base URL; tests point it at a local server.
	telegramAPI = "https://api.telegram.org"
)

func main() {
	botToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID = os.Getenv("TELEGRAM_CHAT_ID")
	if botToken == "" || chatID == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must both be set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/alert", alertHandler)
	mux.HandleFunc("/webhook", webhookHandler)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func alertHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := readBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(string(body))
	if text == "" {
		http.Error(w, "empty message body", http.StatusBadRequest)
		return
	}

	sendOrFail(w, r.Context(), text)
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := readBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	text := extractText(body)
	if text == "" {
		http.Error(w, "empty message body", http.StatusBadRequest)
		return
	}

	sendOrFail(w, r.Context(), text)
}

func readBody(r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("reading request body: %w", err)
	}
	return body, nil
}

// extractText pulls a "text" or "message" string field out of a JSON payload.
// Any non-JSON, or JSON without either field, falls back to the raw body.
func extractText(body []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		for _, key := range []string{"text", "message"} {
			if v, ok := payload[key].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return strings.TrimSpace(string(body))
}

func sendOrFail(w http.ResponseWriter, ctx context.Context, text string) {
	if err := sendTelegram(ctx, text); err != nil {
		log.Printf("telegram send failed: %v", err)
		http.Error(w, "failed to deliver telegram alert", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("sent"))
}

func sendTelegram(ctx context.Context, text string) error {
	payload := map[string]any{
		"text": text,
	}
	if n, err := strconv.ParseInt(chatID, 10, 64); err == nil {
		payload["chat_id"] = n
	} else {
		payload["chat_id"] = chatID
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding telegram payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPI, botToken)
	ctx, cancel := context.WithTimeout(ctx, telegramTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: telegramTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("calling telegram api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("telegram api returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
