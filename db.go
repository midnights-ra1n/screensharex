package main

import (
	"database/sql"
	"strconv"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY,
	username TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password TEXT NOT NULL,
	is_admin INTEGER NOT NULL DEFAULT 0,
	quota INTEGER NOT NULL DEFAULT -1, -- bytes, -1 = server default
	api_token TEXT NOT NULL UNIQUE,
	embed_title TEXT NOT NULL DEFAULT '{filename}',
	embed_description TEXT NOT NULL DEFAULT '{size} • {date}',
	embed_color TEXT NOT NULL DEFAULT '#7c5cff',
	embed_site TEXT NOT NULL DEFAULT 'ScreenShareX',
	embed_raw INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	token TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS media (
	id INTEGER PRIMARY KEY,
	slug TEXT NOT NULL UNIQUE,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	hash TEXT NOT NULL,
	filename TEXT NOT NULL,
	mime TEXT NOT NULL,
	size INTEGER NOT NULL,
	lite_mime TEXT NOT NULL DEFAULT '',
	lite_size INTEGER NOT NULL DEFAULT 0,
	delete_key TEXT NOT NULL,
	views INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS media_user ON media(user_id, created_at);
CREATE INDEX IF NOT EXISTS media_hash ON media(hash);
CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
`

// Server-wide settings editable by admins, with their defaults.
var defaultSettings = map[string]string{
	"registration":         "0",                     // 1 = anyone can create an account
	"default_quota":        strconv.Itoa(1 << 30),   // bytes per user, 0 = unlimited
	"max_upload":           strconv.Itoa(100 << 20), // bytes per file from the web UI
	"max_upload_api":       strconv.Itoa(100 << 20), // bytes per file from desktop/CLI clients
	"uploads_per_hour":     "120",                   // per user from the web UI, 0 = unlimited
	"uploads_per_hour_api": "120",                   // per user from desktop/CLI clients (API token), 0 = unlimited
	"compress":             "1",                     // build lightweight variants with ffmpeg
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // shortcut: one connection, so never query while iterating rows; add a read pool if reads become a bottleneck
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	// Columns added after the first release; the error just means the column already exists.
	db.Exec(`ALTER TABLE media ADD COLUMN via_api INTEGER NOT NULL DEFAULT 0`)
	for k, v := range defaultSettings {
		if _, err := db.Exec(`INSERT OR IGNORE INTO settings(key, value) VALUES(?, ?)`, k, v); err != nil {
			return nil, err
		}
	}
	return db, nil
}

func (a *App) setting(key string) int64 {
	var v string
	a.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}
