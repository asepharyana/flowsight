package store

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

// User is one Google-authenticated account. UserKey (`u:<google_sub>`) is
// the scoping key used by every user-owned table.
type User struct {
	ID        int64
	GoogleSub string
	Email     string
	Name      string
	AvatarURL string
	CreatedAt string
	UserKey   string
}

// UserKeyForSub maps a Google subject to its scoping key.
func UserKeyForSub(sub string) string { return "u:" + sub }

// UpsertUser inserts a first-time login or refreshes profile on return.
func (db *DB) UpsertUser(sub, email, name, avatar string) (*User, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.Exec(`INSERT INTO users(google_sub,email,name,avatar_url,created_at)
		VALUES(?,?,?,?,?)
		ON CONFLICT(google_sub) DO UPDATE SET email=excluded.email,
		name=excluded.name, avatar_url=excluded.avatar_url`,
		sub, email, name, avatar, now); err != nil {
		return nil, err
	}
	return db.UserBySub(sub)
}

// UserBySub returns the user for a Google subject.
func (db *DB) UserBySub(sub string) (*User, error) {
	var u User
	if err := db.QueryRow(`SELECT id,google_sub,email,name,avatar_url,created_at
		FROM users WHERE google_sub=?`, sub).
		Scan(&u.ID, &u.GoogleSub, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt); err != nil {
		return nil, err
	}
	u.UserKey = UserKeyForSub(u.GoogleSub)
	return &u, nil
}

// NewSessionToken mints one opaque 256-bit session token.
func NewSessionToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// CreateSession stores a session valid for ttl.
func (db *DB) CreateSession(userID int64, userKey string, ttl time.Duration) (string, error) {
	tok, err := NewSessionToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO sessions(token,user_key,user_id,expires_at,created_at)
		VALUES(?,?,?,?,?)`, tok, userKey, userID,
		now.Add(ttl).Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
		return "", err
	}
	return tok, nil
}

// SessionUser resolves a session token to its user. Expired tokens are
// deleted lazily and reported invalid.
func (db *DB) SessionUser(token string) (*User, bool) {
	if strings.TrimSpace(token) == "" {
		return nil, false
	}
	var u User
	var exp string
	if err := db.QueryRow(`SELECT u.id,u.google_sub,u.email,u.name,u.avatar_url,
		u.created_at,s.expires_at FROM sessions s
		JOIN users u ON u.id=s.user_id WHERE s.token=?`, token).
		Scan(&u.ID, &u.GoogleSub, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &exp); err != nil {
		return nil, false
	}
	t, err := time.Parse(time.RFC3339, exp)
	if err != nil || time.Now().UTC().After(t) {
		_, _ = db.Exec(`DELETE FROM sessions WHERE token=?`, token)
		return nil, false
	}
	u.UserKey = UserKeyForSub(u.GoogleSub)
	return &u, true
}

// UserKeyBySession returns the scoping key for a valid session token.
func (db *DB) UserKeyBySession(token string) (string, bool) {
	u, ok := db.SessionUser(token)
	if !ok {
		return "", false
	}
	return u.UserKey, true
}

// DeleteSession revokes one session token.
func (db *DB) DeleteSession(token string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE token=?`, token)
	return err
}
