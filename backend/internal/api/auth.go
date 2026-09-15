package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"flowsight/internal/store"
)

// Session lifetime for Google logins.
const sessionTTL = 30 * 24 * time.Hour

// googleAuthURL builds the Google consent URL (state carries platform=web).
func (s *Server) googleAuthURL() string {
	q := url.Values{
		"client_id":     {s.Cfg.GoogleClientID},
		"redirect_uri":  {s.Cfg.GoogleRedirectURL},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"access_type":   {"offline"},
		"prompt":        {"select_account"},
		"state":         {base64.RawURLEncoding.EncodeToString([]byte(`{"platform":"web"}`))},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()
}

// AuthStart serves GET /api/auth/start: 302 to Google, or 404 unconfigured.
func (s *Server) AuthStart(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.HasGoogle() {
		writeErr(w, http.StatusNotFound, "google login not configured")
		return
	}
	http.Redirect(w, r, s.googleAuthURL(), http.StatusFound)
}

// AuthCallback serves GET /api/auth/callback: exchanges code for tokens,
// upserts the user, mints a session cookie, redirects to /.
// Error cases redirect to /login?error=... (web client shows the message).
func (s *Server) AuthCallback(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.HasGoogle() {
		writeErr(w, http.StatusNotFound, "google login not configured")
		return
	}
	fail := func(msg string) {
		http.Redirect(w, r, "/login?error="+url.QueryEscape(msg), http.StatusFound)
	}
	q := r.URL.Query()
	if q.Get("error") != "" || q.Get("code") == "" {
		fail(q.Get("error"))
		if q.Get("error") == "" {
			fail("missing_code")
		}
		return
	}
	tokens, err := exchangeGoogleCode(s.Cfg.GoogleClientID, s.Cfg.GoogleClientSecret,
		s.Cfg.GoogleRedirectURL, q.Get("code"))
	if err != nil {
		fail("google token exchange failed")
		return
	}
	claims, err := decodeGoogleIDToken(tokens.IDToken)
	if err != nil || !claims.EmailVerified || claims.Email == "" {
		fail("email not verified by google")
		return
	}
	name := strings.TrimSpace(claims.Name)
	if name == "" {
		name = strings.Split(claims.Email, "@")[0]
	}
	user, err := s.DB.UpsertUser(claims.Sub, strings.ToLower(strings.TrimSpace(claims.Email)),
		name, claims.Picture)
	if err != nil {
		fail("user store unavailable")
		return
	}
	// Ensure the login user owns the default watchlist on first login.
	if wl, _ := s.DB.Watchlist(user.UserKey); len(wl) == 0 {
		for _, t := range s.Cfg.Watchlist {
			_ = s.DB.AddWatch(user.UserKey, t)
		}
	}
	tok, err := s.DB.CreateSession(user.ID, user.UserKey, sessionTTL)
	if err != nil {
		fail("session store unavailable")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "fs_session", Value: tok, Path: "/", HttpOnly: true,
		Secure: true, SameSite: http.SameSiteLaxMode,
		Expires: time.Now().Add(sessionTTL),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// AuthMe serves GET /api/auth/me: 200 user when logged in, else 401.
func (s *Server) AuthMe(w http.ResponseWriter, r *http.Request) {
	u, ok := s.sessionUser(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "not logged in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id": u.ID, "email": u.Email, "name": u.Name,
			"avatar_url": u.AvatarURL, "user_key": u.UserKey,
			"google_configured": s.Cfg.HasGoogle(),
		},
	})
}

// AuthLogout serves POST /api/auth/logout: revokes the session cookie.
func (s *Server) AuthLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("fs_session"); err == nil {
		_ = s.DB.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: "fs_session", Value: "", Path: "/", HttpOnly: true,
		MaxAge: -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// sessionUser resolves the request's session cookie to its user.
func (s *Server) sessionUser(r *http.Request) (*store.User, bool) {
	c, err := r.Cookie("fs_session")
	if err != nil {
		return nil, false
	}
	return s.DB.SessionUser(c.Value)
}

// googleTokenResp is the subset of oauth2.googleapis.com/token we need.
type googleTokenResp struct {
	IDToken string `json:"id_token"`
}

// googleClaims is the verified-identity subset of the id_token payload.
type googleClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// exchangeGoogleCode swaps an authorization code for tokens.
func exchangeGoogleCode(clientID, secret, redirectURL, code string) (*googleTokenResp, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {secret},
		"redirect_uri":  {redirectURL},
		"grant_type":    {"authorization_code"},
	}
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &url.Error{Op: "exchange", URL: "oauth2.googleapis.com/token", Err: errString(resp.StatusCode)}
	}
	var out googleTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.IDToken == "" {
		return nil, &url.Error{Op: "exchange", URL: "oauth2.googleapis.com/token", Err: errString(0)}
	}
	return &out, nil
}

// decodeGoogleIDToken decodes (not signature-verifies) the id_token payload.
// Full verification happens implicitly: the token arrives over TLS directly
// from Google's token endpoint in exchange for our client_secret.
func decodeGoogleIDToken(idToken string) (*googleClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, errInvalidToken{}
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var c googleClaims
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if c.Sub == "" {
		return nil, errInvalidToken{}
	}
	return &c, nil
}

type errString int

func (e errString) Error() string {
	if e == 0 {
		return "empty id_token"
	}
	return http.StatusText(int(e))
}

type errInvalidToken struct{}

func (errInvalidToken) Error() string { return "invalid id_token" }
