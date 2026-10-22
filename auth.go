package main

import (
	"crypto/rand"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID               int64  `json:"id"`
	Username         string `json:"username"`
	IsAdmin          bool   `json:"is_admin"`
	Quota            int64  `json:"quota"` // -1 = server default
	APIToken         string `json:"api_token,omitempty"`
	EmbedTitle       string `json:"embed_title"`
	EmbedDescription string `json:"embed_description"`
	EmbedColor       string `json:"embed_color"`
	EmbedSite        string `json:"embed_site"`
	EmbedRaw         bool   `json:"embed_raw"`
	CreatedAt        int64  `json:"created_at"`
}

const userCols = `id, username, is_admin, quota, api_token, embed_title, embed_description, embed_color, embed_site, embed_raw, created_at`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	u := &User{}
	err := row.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.Quota, &u.APIToken, &u.EmbedTitle,
		&u.EmbedDescription, &u.EmbedColor, &u.EmbedSite, &u.EmbedRaw, &u.CreatedAt)
	return u, err
}

func randString(n int) string { return rand.Text()[:n] } // base32, 26 chars max

const sessionCookie = "ssx_session"

// authUser resolves the caller from the session cookie, a Bearer token, or a "token" form/query field
// (some capture tools can only send form fields). viaToken is true for desktop/CLI clients, false for the web UI.
func (a *App) authUser(r *http.Request) (u *User, viaToken bool) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		u, err := scanUser(a.db.QueryRow(`SELECT `+userCols+` FROM users WHERE id = (SELECT user_id FROM sessions WHERE token = ? AND expires_at > ?)`,
			c.Value, time.Now().Unix()))
		if err == nil {
			return u, false
		}
	}
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if tok == "" {
		tok = r.Header.Get("X-API-Key")
	}
	if tok == "" {
		tok = r.FormValue("token")
	}
	if tok == "" {
		return nil, false
	}
	u, err := scanUser(a.db.QueryRow(`SELECT `+userCols+` FROM users WHERE api_token = ?`, tok))
	if err != nil {
		return nil, false
	}
	return u, true
}

type userHandler func(http.ResponseWriter, *http.Request, *User)

func (a *App) user(h userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := a.authUser(r)
		if u == nil {
			fail(w, http.StatusUnauthorized, "Non connecté")
			return
		}
		h(w, r, u)
	}
}

func (a *App) admin(h userHandler) http.HandlerFunc {
	return a.user(func(w http.ResponseWriter, r *http.Request, u *User) {
		if !u.IsAdmin {
			fail(w, http.StatusForbidden, "Réservé aux administrateurs")
			return
		}
		h(w, r, u)
	})
}

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)

func (a *App) createAccount(username, password string, admin bool) (int64, string) {
	if !usernameRe.MatchString(username) {
		return 0, "Nom d'utilisateur : 3 à 32 caractères (lettres, chiffres, _ . -)"
	}
	if len(password) < 8 || len(password) > 72 {
		return 0, "Mot de passe : 8 à 72 caractères"
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	res, err := a.db.Exec(`INSERT INTO users(username, password, is_admin, api_token, created_at) VALUES(?, ?, ?, ?, ?)`,
		username, hash, admin, rand.Text(), time.Now().Unix())
	if err != nil {
		return 0, "Ce nom d'utilisateur est déjà pris"
	}
	id, _ := res.LastInsertId()
	return id, ""
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !readJSON(w, r, &c) {
		return
	}
	var users int
	a.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users)
	// The very first account is the administrator, even when registration is closed.
	if users > 0 && a.setting("registration") != 1 {
		fail(w, http.StatusForbidden, "Les inscriptions sont fermées")
		return
	}
	id, msg := a.createAccount(c.Username, c.Password, users == 0)
	if msg != "" {
		fail(w, http.StatusBadRequest, msg)
		return
	}
	a.startSession(w, r, id)
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !readJSON(w, r, &c) {
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !a.logins.allow(ip) {
		fail(w, http.StatusTooManyRequests, "Trop de tentatives, réessayez dans quelques minutes")
		return
	}
	var id int64
	var hash []byte
	err := a.db.QueryRow(`SELECT id, password FROM users WHERE username = ?`, c.Username).Scan(&id, &hash)
	if err != nil || bcrypt.CompareHashAndPassword(hash, []byte(c.Password)) != nil {
		a.logins.failed(ip)
		fail(w, http.StatusUnauthorized, "Identifiants incorrects")
		return
	}
	a.startSession(w, r, id)
}

func (a *App) startSession(w http.ResponseWriter, r *http.Request, userID int64) {
	tok := rand.Text()
	exp := time.Now().Add(30 * 24 * time.Hour)
	a.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().Unix())
	if _, err := a.db.Exec(`INSERT INTO sessions(token, user_id, expires_at) VALUES(?, ?, ?)`, tok, userID, exp.Unix()); err != nil {
		fail(w, 500, err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: tok, Path: "/", Expires: exp, HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: strings.HasPrefix(a.base(r), "https")})
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.db.Exec(`DELETE FROM sessions WHERE token = ?`, c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1})
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *App) quotaOf(u *User) int64 {
	if u.Quota >= 0 {
		return u.Quota
	}
	return a.setting("default_quota")
}

func (a *App) me(w http.ResponseWriter, r *http.Request, u *User) {
	var used, count int64
	a.db.QueryRow(`SELECT COALESCE(SUM(size), 0), COUNT(*) FROM media WHERE user_id = ?`, u.ID).Scan(&used, &count)
	writeJSON(w, 200, map[string]any{"user": u, "used": used, "count": count, "quota": a.quotaOf(u),
		"max_upload": a.setting("max_upload"), "base_url": a.base(r)})
}

var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (a *App) updateMe(w http.ResponseWriter, r *http.Request, u *User) {
	var in struct {
		User
		Password        string `json:"password"`
		CurrentPassword string `json:"current_password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if in.Password != "" {
		var hash []byte
		a.db.QueryRow(`SELECT password FROM users WHERE id = ?`, u.ID).Scan(&hash)
		if bcrypt.CompareHashAndPassword(hash, []byte(in.CurrentPassword)) != nil {
			fail(w, 400, "Mot de passe actuel incorrect")
			return
		}
		if len(in.Password) < 8 || len(in.Password) > 72 {
			fail(w, 400, "Mot de passe : 8 à 72 caractères")
			return
		}
		h, _ := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		a.db.Exec(`UPDATE users SET password = ? WHERE id = ?`, h, u.ID)
		a.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, u.ID)
		a.startSession(w, r, u.ID)
		return
	}
	if !colorRe.MatchString(in.EmbedColor) || len(in.EmbedTitle) > 200 || len(in.EmbedDescription) > 500 || len(in.EmbedSite) > 100 {
		fail(w, 400, "Paramètres d'embed invalides")
		return
	}
	a.db.Exec(`UPDATE users SET embed_title = ?, embed_description = ?, embed_color = ?, embed_site = ?, embed_raw = ? WHERE id = ?`,
		in.EmbedTitle, in.EmbedDescription, in.EmbedColor, in.EmbedSite, in.EmbedRaw, u.ID)
	a.forgetUserMedia(u.ID)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *App) regenToken(w http.ResponseWriter, r *http.Request, u *User) {
	tok := rand.Text()
	a.db.Exec(`UPDATE users SET api_token = ? WHERE id = ?`, tok, u.ID)
	writeJSON(w, 200, map[string]string{"api_token": tok})
}

// forgetUserMedia drops cached views of a user's media (their embed settings are baked in).
func (a *App) forgetUserMedia(userID int64) {
	rows, err := a.db.Query(`SELECT slug FROM media WHERE user_id = ?`, userID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		rows.Scan(&s)
		a.cache.Del("m:" + s)
	}
}

// loginLimiter allows 10 failed logins per IP per 15 minutes.
type loginLimiter struct {
	mu sync.Mutex
	m  map[string][]time.Time
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{m: map[string][]time.Time{}} }

func (l *loginLimiter) recent(ip string) []time.Time {
	var keep []time.Time
	for _, t := range l.m[ip] {
		if time.Since(t) < 15*time.Minute {
			keep = append(keep, t)
		}
	}
	if keep == nil {
		delete(l.m, ip)
	} else {
		l.m[ip] = keep
	}
	return keep
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(ip)) < 10
}

func (l *loginLimiter) failed(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.m[ip] = append(l.recent(ip), time.Now())
}
