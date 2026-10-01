package auth

import (
	"sync"
	"time"
)

const (
	maxLoginFailures = 5
	loginWindow      = 15 * time.Minute
)

type LoginLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
	now      func() time.Time
}

func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{failures: make(map[string][]time.Time), now: time.Now}
}

func (limiter *LoginLimiter) Allowed(ip string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return len(limiter.activeFailures(ip)) < maxLoginFailures
}

func (limiter *LoginLimiter) RecordFailure(ip string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	failures := limiter.activeFailures(ip)
	limiter.failures[ip] = append(failures, limiter.now())
}

func (limiter *LoginLimiter) Reset(ip string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	delete(limiter.failures, ip)
}

func (limiter *LoginLimiter) activeFailures(ip string) []time.Time {
	cutoff := limiter.now().Add(-loginWindow)
	active := limiter.failures[ip][:0]
	for _, failure := range limiter.failures[ip] {
		if !failure.Before(cutoff) {
			active = append(active, failure)
		}
	}
	if len(active) == 0 {
		delete(limiter.failures, ip)
		return nil
	}
	limiter.failures[ip] = active
	return active
}
