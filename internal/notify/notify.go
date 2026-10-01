// Package notify sends inquiry email notifications through Azure
// Communication Services (ACS) Email, using the REST API signed with
// HMAC-SHA256 (there is no stable official Go SDK for ACS Email yet).
package notify

import (
	"context"
)

// InquiryNotification holds the values needed to email a new contact
// inquiry. It is independent of models.Inquiry so the notifier only
// depends on the fields it actually sends.
type InquiryNotification struct {
	Name     string
	Email    string
	Subject  string
	Message  string
	AdminURL string
}

// Notifier is implemented by anything that can deliver an inquiry
// notification. In Go, an interface is just a set of method signatures:
// any type with a matching Notify method satisfies it automatically,
// with no explicit "implements" declaration. That lets production code
// depend on this interface while tests substitute a fake, in-memory
// implementation instead of making real network calls.
type Notifier interface {
	Notify(ctx context.Context, notification InquiryNotification) error
}

// NoopNotifier is used when email notifications are not configured
// (missing env vars). It satisfies the Notifier interface but does
// nothing, so callers never need a nil check.
type NoopNotifier struct{}

func (NoopNotifier) Notify(context.Context, InquiryNotification) error {
	return nil
}
