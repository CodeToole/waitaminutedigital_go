package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const acsEmailAPIVersion = "2023-03-31"

// ACSNotifier sends inquiry emails through the Azure Communication
// Services Email REST API, authenticating each request with an
// HMAC-SHA256 signature computed from the access key.
type ACSNotifier struct {
	endpoint   string
	accessKey  string
	from       string
	to         string
	httpClient *http.Client
}

// NewACSNotifier builds a notifier for the given ACS resource. endpoint is
// the resource's base URL (e.g. https://xyz.communication.azure.com),
// accessKey is the resource access key, from is the verified sender
// address, and to is the recipient that gets notified of new inquiries.
func NewACSNotifier(endpoint string, accessKey string, from string, to string) *ACSNotifier {
	return &ACSNotifier{
		endpoint:  strings.TrimRight(endpoint, "/"),
		accessKey: accessKey,
		from:      from,
		to:        to,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type acsEmailAddress struct {
	Address string `json:"address"`
}

type acsEmailRecipients struct {
	To []acsEmailAddress `json:"to"`
}

type acsEmailContent struct {
	Subject   string `json:"subject"`
	PlainText string `json:"plainText"`
}

type acsSendEmailRequest struct {
	SenderAddress string             `json:"senderAddress"`
	Recipients    acsEmailRecipients `json:"recipients"`
	Content       acsEmailContent    `json:"content"`
	ReplyTo       []acsEmailAddress  `json:"replyTo,omitempty"`
}

// Notify sends a plain-text email describing the new inquiry, with
// reply-to set to the visitor so replying from an inbox goes straight
// back to them.
func (n *ACSNotifier) Notify(ctx context.Context, notification InquiryNotification) error {
	body := acsSendEmailRequest{
		SenderAddress: n.from,
		Recipients:    acsEmailRecipients{To: []acsEmailAddress{{Address: n.to}}},
		Content: acsEmailContent{
			Subject:   "New inquiry: " + notification.Subject,
			PlainText: plainTextBody(notification),
		},
		ReplyTo: []acsEmailAddress{{Address: notification.Email}},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal email request: %w", err)
	}

	req, err := n.newSignedRequest(ctx, payload)
	if err != nil {
		return fmt.Errorf("build signed request: %w", err)
	}

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send email request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("acs email send failed: status %d: %s", resp.StatusCode, responseBody)
	}
	return nil
}

func plainTextBody(notification InquiryNotification) string {
	return fmt.Sprintf(
		"New inquiry from the Waitaminute Digital contact form.\n\n"+
			"Name: %s\nEmail: %s\nSubject: %s\n\nMessage:\n%s\n\nView in admin: %s\n",
		notification.Name, notification.Email, notification.Subject, notification.Message, notification.AdminURL,
	)
}

// newSignedRequest builds the POST request to the ACS email send endpoint
// and signs it per Azure's HMAC authentication scheme: the Authorization
// header is computed from the HTTP method, path+query, date, host, and a
// hash of the body, signed with the resource's access key.
func (n *ACSNotifier) newSignedRequest(ctx context.Context, payload []byte) (*http.Request, error) {
	requestURL := n.endpoint + "/emails:send?api-version=" + acsEmailAPIVersion
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}

	date := time.Now().UTC().Format(http.TimeFormat)
	contentHash := hashBody(payload)

	signature, err := n.sign(parsed.RequestURI(), date, parsed.Host, contentHash)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-ms-date", date)
	req.Header.Set("x-ms-content-sha256", contentHash)
	req.Header.Set("Authorization", fmt.Sprintf(
		"HMAC-SHA256 SignedHeaders=x-ms-date;host;x-ms-content-sha256&Signature=%s", signature,
	))
	return req, nil
}

func (n *ACSNotifier) sign(pathAndQuery string, date string, host string, contentHash string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(n.accessKey)
	if err != nil {
		return "", fmt.Errorf("decode access key: %w", err)
	}
	stringToSign := fmt.Sprintf("POST\n%s\n%s;%s;%s", pathAndQuery, date, host, contentHash)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func hashBody(payload []byte) string {
	sum := sha256.Sum256(payload)
	return base64.StdEncoding.EncodeToString(sum[:])
}
