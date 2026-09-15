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
	// Ensure the login user owns the "semua" default watchlist on first login.
	if wl, _ := s.DB.Watchlist(user.UserKey); len(wl) == 0 {
		seeds, _ := s.DB.AllTickers()
		if len(seeds) == 0 {
			seeds = s.Cfg.Watchlist
		}
		for _, t := range seeds {
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

// requireLogin is chi middleware: 401 unless a valid session cookie is
// present. No X-User-Key/demo fallback — fitur dan filter milik user yg
// login. Dashboard (flow/*, briefing) tetap publik di luar grup ini.
func (s *Server) requireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.sessionUser(r); !ok {
			writeErr(w, http.StatusUnauthorized, "login dulu untuk pakai fitur ini")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// localCreds decodes {username, password} with shared validation.
func localCreds(r *http.Request) (string, string, bool) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return "", "", false
	}
	return strings.ToLower(strings.TrimSpace(req.Username)), req.Password, true
}

// mintSession creates a session + sets the fs_session cookie.
func (s *Server) mintSession(w http.ResponseWriter, user *store.User) bool {
	tok, err := s.DB.CreateSession(user.ID, user.UserKey, sessionTTL)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "session store unavailable")
		return false
	}
	http.SetCookie(w, &http.Cookie{
		Name: "fs_session", Value: tok, Path: "/", HttpOnly: true,
		Secure: true, SameSite: http.SameSiteLaxMode,
		Expires: time.Now().Add(sessionTTL),
	})
	return true
}

// userJSON renders the /api/auth/me shape for one user.
func (s *Server) userJSON(u *store.User) map[string]any {
	return map[string]any{
		"id": u.ID, "email": u.Email, "name": u.Name,
		"avatar_url": u.AvatarURL, "user_key": u.UserKey,
		"google_configured": s.Cfg.HasGoogle(),
	}
}

// seedWatchlistSemua gives a fresh user the "semua" default: every ticker
// with stored data (fallback: config watchlist, last: BBCA).
func (s *Server) seedWatchlistSemua(userKey string) {
	if wl, _ := s.DB.Watchlist(userKey); len(wl) > 0 {
		return
	}
	seeds, _ := s.DB.AllTickers()
	if len(seeds) == 0 {
		seeds = s.Cfg.Watchlist
	}
	if len(seeds) == 0 {
		seeds = []string{"BBCA"}
	}
	for _, t := range seeds {
		_ = s.DB.AddWatch(userKey, t)
	}
}

// AuthSignup serves POST /api/auth/signup {username, password}: registers a
// local account, seeds the "semua" watchlist, logs in immediately.
func (s *Server) AuthSignup(w http.ResponseWriter, r *http.Request) {
	username, password, ok := localCreds(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, err := s.DB.CreateLocalUser(username, password)
	if err != nil {
		switch err {
		case store.ErrTaken:
			writeErr(w, http.StatusConflict, err.Error())
		case store.ErrBadUsername, store.ErrBadPassword:
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
		default:
			writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		}
		return
	}
	s.seedWatchlistSemua(user.UserKey)
	if !s.mintSession(w, user) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": s.userJSON(user)})
}

// AuthLogin serves POST /api/auth/login {username, password}: verifies the
// local account and mints a session.
func (s *Server) AuthLogin(w http.ResponseWriter, r *http.Request) {
	username, password, ok := localCreds(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, err := s.DB.CheckLocalUser(username, password)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "username atau password salah")
		return
	}
	s.seedWatchlistSemua(user.UserKey)
	if !s.mintSession(w, user) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": s.userJSON(user)})
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
