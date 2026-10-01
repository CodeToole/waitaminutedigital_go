package auth

import (
	"testing"
	"time"
)

func TestLoginLimiterUsesEnvironmentSpecificWindow(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		production  bool
		advance     time.Duration
		wantAllowed bool
	}{
		{name: "development unlocks after one minute", production: false, advance: time.Minute + time.Nanosecond, wantAllowed: true},
		{name: "production remains locked before fifteen minutes", production: true, advance: time.Minute + time.Nanosecond, wantAllowed: false},
		{name: "production unlocks after fifteen minutes", production: true, advance: 15*time.Minute + time.Nanosecond, wantAllowed: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			limiter := NewLoginLimiter(tc.production)
			limiter.now = func() time.Time { return now }
			for range maxLoginFailures {
				limiter.RecordFailure("192.0.2.4")
			}
			if limiter.Allowed("192.0.2.4") {
				t.Fatal("limiter allowed login before its window elapsed")
			}
			now = now.Add(tc.advance)
			if got := limiter.Allowed("192.0.2.4"); got != tc.wantAllowed {
				t.Errorf("Allowed() = %v, want %v", got, tc.wantAllowed)
			}
		})
	}
}
