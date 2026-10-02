package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/alexedwards/scs/v2"
)

// hmacSessionStore derives opaque store keys from SCS's random session
// tokens. A leaked store then cannot be used to recover usable browser
// tokens without the application's SESSION_SECRET.
type hmacSessionStore struct {
	store scs.Store
	key   []byte
}

func NewHMACSessionStore(store scs.Store, key []byte) scs.Store {
	return hmacSessionStore{store: store, key: append([]byte(nil), key...)}
}

func (store hmacSessionStore) Delete(token string) error {
	return store.store.Delete(store.keyToken(token))
}

func (store hmacSessionStore) Find(token string) ([]byte, bool, error) {
	return store.store.Find(store.keyToken(token))
}

func (store hmacSessionStore) Commit(token string, data []byte, expiry time.Time) error {
	return store.store.Commit(store.keyToken(token), data, expiry)
}

func (store hmacSessionStore) keyToken(token string) string {
	mac := hmac.New(sha256.New, store.key)
	_, _ = mac.Write([]byte(token))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
