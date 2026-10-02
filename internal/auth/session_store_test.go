package auth

import (
	"bytes"
	"testing"
	"time"
)

type recordingSessionStore struct {
	token string
	data  []byte
}

func (store *recordingSessionStore) Delete(token string) error {
	store.token = token
	store.data = nil
	return nil
}

func (store *recordingSessionStore) Find(token string) ([]byte, bool, error) {
	if token != store.token || store.data == nil {
		return nil, false, nil
	}
	return store.data, true, nil
}

func (store *recordingSessionStore) Commit(token string, data []byte, _ time.Time) error {
	store.token = token
	store.data = append([]byte(nil), data...)
	return nil
}

func TestHMACSessionStoreDoesNotStoreBrowserToken(t *testing.T) {
	const sessionToken = "random-scs-session-token"
	inner := &recordingSessionStore{}
	store := NewHMACSessionStore(inner, []byte("0123456789abcdef0123456789abcdef"))
	if err := store.Commit(sessionToken, []byte("session"), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if inner.token == sessionToken {
		t.Fatal("session store retained the browser token instead of its HMAC")
	}
	data, found, err := store.Find(sessionToken)
	if err != nil || !found || !bytes.Equal(data, []byte("session")) {
		t.Fatalf("Find() = %q, %v, %v; want session data", data, found, err)
	}
}
