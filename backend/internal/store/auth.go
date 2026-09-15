package store

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User is one Google-authenticated account. UserKey (`u:<google_sub>`) is
// the scoping key used by every user-owned table. Local accounts
// (username+password signup) store google_sub='local:<username>' with
// username+password_hash set, keeping session scoping unchanged.
type authErr string

func (e authErr) Error() string { return string(e) }

const (
	ErrBadLogin    authErr = "username atau password salah"
	ErrBadUsername authErr = "username 3-32 karakter: huruf, angka, titik, _ -"
	ErrBadPassword authErr = "password minimal 8 karakter"
	ErrTaken       authErr = "username sudah dipakai"
)

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

// localSub maps a username to its google_sub value for local accounts.
func localSub(username string) string { return "local:" + strings.ToLower(username) }

// CreateLocalUser registers a username+password account (bcrypt cost 10).
// Username: 3-32 chars [a-z0-9._-]; password: min 8 chars.
func (db *DB) CreateLocalUser(username, password string) (*User, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if !validUsername(username) {
		return nil, ErrBadUsername
	}
	if len(password) < 8 {
		return nil, ErrBadPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return nil, err
	}
	sub := localSub(username)
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.Exec(`INSERT INTO users(google_sub,email,name,username,password_hash,created_at)
		VALUES(?,?,?,?,?,?)`, sub, username, username, username, string(hash), now); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrTaken
		}
		return nil, err
	}
	return db.UserBySub(sub)
}

// CheckLocalUser verifies username+password, returning the user on success.
func (db *DB) CheckLocalUser(username, password string) (*User, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	var u User
	var hash string
	if err := db.QueryRow(`SELECT id,google_sub,email,name,avatar_url,created_at,password_hash
		FROM users WHERE username=?`, username).
		Scan(&u.ID, &u.GoogleSub, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &hash); err != nil {
		return nil, ErrBadLogin
	}
	if hash == "" {
		return nil, ErrBadLogin // Google-only account, no local password
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, ErrBadLogin
	}
	u.UserKey = UserKeyForSub(u.GoogleSub)
	return &u, nil
}

// validUsername allows 3-32 chars of lowercase letters, digits, . _ -.
func validUsername(u string) bool {
	if len(u) < 3 || len(u) > 32 {
		return false
	}
	for _, c := range u {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}
