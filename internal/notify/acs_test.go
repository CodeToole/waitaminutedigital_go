package notify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestACSNotifierSendsSignedRequest(t *testing.T) {
	const accessKey = "c2VjcmV0LWtleS1iYXNlNjQ=" // base64("secret-key-base64")

	var gotRequest *http.Request
	var gotBody acsSendEmailRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequest = r
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	notifier := NewACSNotifier(server.URL, accessKey, "noreply@waitaminutedigital.com", "corneliustoole@waitaminutedigital.com")
	err := notifier.Notify(context.Background(), InquiryNotification{
		Name:     "Neil",
		Email:    "visitor@example.com",
		Subject:  "Custom Software",
		Message:  "I need a small tool.",
		AdminURL: "https://waitaminutedigital.com/admin/inquiries",
	})
	if err != nil {
		t.Fatalf("Notify() returned error: %v", err)
	}

	if gotRequest.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", gotRequest.Method)
	}
	if !strings.HasPrefix(gotRequest.URL.Path, "/emails:send") {
		t.Errorf("path = %q, want prefix /emails:send", gotRequest.URL.Path)
	}
	if gotRequest.URL.Query().Get("api-version") != acsEmailAPIVersion {
		t.Errorf("api-version = %q, want %q", gotRequest.URL.Query().Get("api-version"), acsEmailAPIVersion)
	}
	if gotRequest.Header.Get("x-ms-date") == "" {
		t.Error("x-ms-date header missing")
	}
	if gotRequest.Header.Get("x-ms-content-sha256") == "" {
		t.Error("x-ms-content-sha256 header missing")
	}
	auth := gotRequest.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "HMAC-SHA256 SignedHeaders=x-ms-date;host;x-ms-content-sha256&Signature=") {
		t.Errorf("Authorization header malformed: %q", auth)
	}
	signature := strings.TrimPrefix(auth, "HMAC-SHA256 SignedHeaders=x-ms-date;host;x-ms-content-sha256&Signature=")
	if _, err := base64.StdEncoding.DecodeString(signature); err != nil {
		t.Errorf("signature is not valid base64: %v", err)
	}

	if gotBody.SenderAddress != "noreply@waitaminutedigital.com" {
		t.Errorf("senderAddress = %q", gotBody.SenderAddress)
	}
	if len(gotBody.Recipients.To) != 1 || gotBody.Recipients.To[0].Address != "corneliustoole@waitaminutedigital.com" {
		t.Errorf("recipients.to = %+v", gotBody.Recipients.To)
	}
	if len(gotBody.ReplyTo) != 1 || gotBody.ReplyTo[0].Address != "visitor@example.com" {
		t.Errorf("replyTo = %+v", gotBody.ReplyTo)
	}
	if !strings.Contains(gotBody.Content.PlainText, "I need a small tool.") {
		t.Errorf("plain text body missing message: %q", gotBody.Content.PlainText)
	}
}

func TestACSNotifierReturnsErrorOnFailureStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad signature"}}`))
	}))
	defer server.Close()

	notifier := NewACSNotifier(server.URL, base64.StdEncoding.EncodeToString([]byte("key")), "from@example.com", "to@example.com")
	err := notifier.Notify(context.Background(), InquiryNotification{Email: "visitor@example.com"})
	if err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}
