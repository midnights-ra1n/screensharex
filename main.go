package main

import (
	"database/sql"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/midnights-ra1n/screensharex/web"
)

// Set at build time with -ldflags "-X main.version=v1.2.3".
var (
	version = "dev"
	repo    = "midnights-ra1n/screensharex"
)

type App struct {
	db      *sql.DB
	cache   Cache
	dataDir string
	baseURL string // optional, otherwise derived from the request
	started time.Time
	litesem chan struct{} // limits concurrent ffmpeg jobs
	logins  *loginLimiter
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		println(version)
		return
	}
	dataDir := env("SSX_DATA", "./data")
	app, err := newApp(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	app.baseURL = strings.TrimSuffix(os.Getenv("SSX_BASE_URL"), "/")
	if r := os.Getenv("SSX_REDIS"); r != "" {
		c, err := newRespCache(r)
		if err != nil {
			log.Fatalf("redis cache %s: %v", r, err)
		}
		app.cache = c
		log.Printf("cache: redis-compatible server at %s", r)
	}
	addr := env("SSX_ADDR", ":8080")
	log.Printf("ScreenShareX %s listening on %s (data: %s)", version, addr, dataDir)
	srv := &http.Server{Addr: addr, Handler: app.routes(), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func newApp(dataDir string) (*App, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, "files"), 0o750); err != nil {
		return nil, err
	}
	db, err := openDB(filepath.Join(dataDir, "screensharex.db"))
	if err != nil {
		return nil, err
	}
	return &App{db: db, cache: newMemCache(), dataDir: dataDir, started: time.Now(),
		litesem: make(chan struct{}, 1), logins: newLoginLimiter()}, nil
}

func (a *App) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/public", a.public)
	m.HandleFunc("POST /api/auth/register", a.register)
	m.HandleFunc("POST /api/auth/login", a.login)
	m.HandleFunc("POST /api/auth/logout", a.logout)
	m.HandleFunc("GET /api/oembed", a.oembed)
	m.HandleFunc("POST /api/upload", a.upload) // session or API token

	m.HandleFunc("GET /api/me", a.user(a.me))
	m.HandleFunc("PATCH /api/me", a.user(a.updateMe))
	m.HandleFunc("POST /api/me/token", a.user(a.regenToken))
	m.HandleFunc("GET /api/media", a.user(a.listMedia))
	m.HandleFunc("DELETE /api/media/{id}", a.user(a.deleteMedia))
	m.HandleFunc("GET /api/stats", a.user(a.stats))
	m.HandleFunc("GET /api/clients/{name}", a.user(a.clientConfig))

	m.HandleFunc("GET /api/admin/settings", a.admin(a.getSettings))
	m.HandleFunc("PUT /api/admin/settings", a.admin(a.putSettings))
	m.HandleFunc("GET /api/admin/users", a.admin(a.listUsers))
	m.HandleFunc("POST /api/admin/users", a.admin(a.createUser))
	m.HandleFunc("PATCH /api/admin/users/{id}", a.admin(a.updateUser))
	m.HandleFunc("DELETE /api/admin/users/{id}", a.admin(a.deleteUser))
	m.HandleFunc("GET /api/admin/export", a.admin(a.export))
	m.HandleFunc("POST /api/admin/import", a.admin(a.importArchive))
	m.HandleFunc("GET /api/admin/update", a.admin(a.checkUpdate))
	m.HandleFunc("POST /api/admin/update", a.admin(a.installUpdate))

	m.HandleFunc("GET /{slug}", a.view)
	// One catch-all so that /assets/ stays strictly more specific than the media routes.
	m.HandleFunc("GET /{slug}/{rest...}", func(w http.ResponseWriter, r *http.Request) {
		switch rest := r.PathValue("rest"); {
		case rest == "raw" || rest == "lite":
			a.raw(w, r)
		case strings.HasPrefix(rest, "delete/"):
			a.deleteConfirm(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	m.HandleFunc("POST /{slug}/delete/{key}", a.deleteByKey)
	m.HandleFunc("DELETE /{slug}/delete/{key}", a.deleteByKey)

	dist, _ := fs.Sub(web.Dist, "dist")
	files := http.FileServerFS(dist)
	m.Handle("GET /assets/", files)
	m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, dist, "index.html")
	})
	for _, f := range []string{"favicon.svg", "manifest.webmanifest"} {
		m.Handle("GET /"+f, files)
	}
	return m
}

func (a *App) base(r *http.Request) string {
	if a.baseURL != "" {
		return a.baseURL
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "JSON invalide")
		return false
	}
	return true
}

func (a *App) public(w http.ResponseWriter, r *http.Request) {
	var users int
	a.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users)
	writeJSON(w, 200, map[string]any{
		"version":      version,
		"registration": a.setting("registration") == 1 || users == 0,
		"setup":        users == 0,
	})
}
