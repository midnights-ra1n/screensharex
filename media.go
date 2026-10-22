package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Media struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	UserID    int64  `json:"user_id"`
	Hash      string `json:"hash"`
	Filename  string `json:"filename"`
	Mime      string `json:"mime"`
	Size      int64  `json:"size"`
	LiteMime  string `json:"lite_mime"`
	LiteSize  int64  `json:"lite_size"`
	DeleteKey string `json:"-"`
	Views     int64  `json:"views"`
	CreatedAt int64  `json:"created_at"`
}

const mediaCols = `id, slug, user_id, hash, filename, mime, size, lite_mime, lite_size, delete_key, views, created_at`

func scanMedia(row interface{ Scan(...any) error }) (*Media, error) {
	m := &Media{}
	err := row.Scan(&m.ID, &m.Slug, &m.UserID, &m.Hash, &m.Filename, &m.Mime, &m.Size, &m.LiteMime, &m.LiteSize, &m.DeleteKey, &m.Views, &m.CreatedAt)
	return m, err
}

func (a *App) filePath(hash string) string { return filepath.Join(a.dataDir, "files", hash[:2], hash) }

// inline reports whether a type is safe to render in the browser; everything else is a download.
// SVG is excluded because it can carry scripts.
func inline(mime string) bool {
	return (strings.HasPrefix(mime, "image/") && !strings.Contains(mime, "svg")) ||
		strings.HasPrefix(mime, "video/") || strings.HasPrefix(mime, "audio/")
}

func slugify() string {
	const alphabet = "abcdefghijkmnpqrstuvwxyz23456789" // 32 symbols, no look-alikes
	b := []byte(strings.ToLower(randString(8)))
	for i := range b {
		b[i] = alphabet[strings.IndexByte("abcdefghijklmnopqrstuvwxyz234567", b[i])]
	}
	return string(b)
}

func (a *App) mediaURLs(r *http.Request, m *Media) map[string]any {
	b := a.base(r) + "/" + m.Slug
	return map[string]any{"url": b, "raw": b + "/raw", "thumb": b + "/lite", "delete_url": b + "/delete/" + m.DeleteKey}
}

func (a *App) upload(w http.ResponseWriter, r *http.Request) {
	maxSize := a.setting("max_upload")
	r.Body = http.MaxBytesReader(w, r.Body, maxSize+1<<20) // + room for multipart overhead
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		fail(w, http.StatusRequestEntityTooLarge, "Fichier trop volumineux ou requête invalide")
		return
	}
	defer r.MultipartForm.RemoveAll()
	u := a.authUser(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "Jeton invalide")
		return
	}
	fh := firstFile(r)
	if fh == nil {
		fail(w, 400, "Aucun fichier (champ \"file\")")
		return
	}
	if fh.Size > maxSize {
		fail(w, http.StatusRequestEntityTooLarge, "Fichier trop volumineux (max "+human(maxSize)+")")
		return
	}
	if limit := a.setting("uploads_per_hour"); limit > 0 {
		var n int64
		a.db.QueryRow(`SELECT COUNT(*) FROM media WHERE user_id = ? AND created_at > ?`, u.ID, time.Now().Add(-time.Hour).Unix()).Scan(&n)
		if n >= limit {
			fail(w, http.StatusTooManyRequests, fmt.Sprintf("Limite de %d envois par heure atteinte", limit))
			return
		}
	}
	f, err := fh.Open()
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	defer f.Close()

	tmp, err := os.CreateTemp(filepath.Join(a.dataDir, "files"), "upload-*")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	h := sha256.New()
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	_, err = io.Copy(io.MultiWriter(tmp, h), io.MultiReader(bytes.NewReader(head), f))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	hash := hex.EncodeToString(h.Sum(nil))

	// Same file already uploaded by this user: hand back the existing link.
	if m, err := scanMedia(a.db.QueryRow(`SELECT `+mediaCols+` FROM media WHERE user_id = ? AND hash = ?`, u.ID, hash)); err == nil {
		res := a.mediaURLs(r, m)
		res["media"], res["duplicate"] = m, true
		writeJSON(w, 200, res)
		return
	}

	if q := a.quotaOf(u); q > 0 {
		var used int64
		a.db.QueryRow(`SELECT COALESCE(SUM(size), 0) FROM media WHERE user_id = ?`, u.ID).Scan(&used)
		if used+fh.Size > q {
			fail(w, http.StatusRequestEntityTooLarge, "Quota dépassé ("+human(used)+" / "+human(q)+")")
			return
		}
	}

	ctype := http.DetectContentType(head)
	if ctype == "application/octet-stream" || strings.HasPrefix(ctype, "text/plain") {
		if t := mime.TypeByExtension(filepath.Ext(fh.Filename)); t != "" {
			ctype = t
		}
	}
	ctype, _, _ = strings.Cut(ctype, ";")

	// Content-addressed storage: identical files from different users share one blob on disk.
	dst := a.filePath(hash)
	os.MkdirAll(filepath.Dir(dst), 0o750)
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		if err := os.Rename(tmp.Name(), dst); err != nil {
			fail(w, 500, err.Error())
			return
		}
	}

	m := &Media{UserID: u.ID, Hash: hash, Filename: filepath.Base(fh.Filename), Mime: ctype, Size: fh.Size,
		DeleteKey: randString(20), CreatedAt: time.Now().Unix()}
	a.db.QueryRow(`SELECT lite_mime, lite_size FROM media WHERE hash = ? LIMIT 1`, hash).Scan(&m.LiteMime, &m.LiteSize)
	for range 3 { // retry on the (unlikely) slug collision
		m.Slug = slugify()
		var res sql.Result
		res, err = a.db.Exec(`INSERT INTO media(slug, user_id, hash, filename, mime, size, lite_mime, lite_size, delete_key, created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			m.Slug, m.UserID, m.Hash, m.Filename, m.Mime, m.Size, m.LiteMime, m.LiteSize, m.DeleteKey, m.CreatedAt)
		if err == nil {
			m.ID, _ = res.LastInsertId()
			break
		}
	}
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if m.LiteMime == "" && a.setting("compress") == 1 {
		go a.makeLite(hash, ctype, m.Size)
	}
	res := a.mediaURLs(r, m) // flat url/thumb/delete_url keys: easiest for capture tools to parse
	res["media"] = m
	writeJSON(w, 200, res)
}

// firstFile prefers the "file" field but accepts whatever field name a client uses.
func firstFile(r *http.Request) *multipart.FileHeader {
	if fs := r.MultipartForm.File["file"]; len(fs) > 0 {
		return fs[0]
	}
	for _, fs := range r.MultipartForm.File {
		if len(fs) > 0 {
			return fs[0]
		}
	}
	return nil
}

func (a *App) listMedia(w http.ResponseWriter, r *http.Request, u *User) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	where, args := `user_id = ?`, []any{u.ID}
	if q := r.URL.Query().Get("q"); q != "" {
		where += ` AND filename LIKE ?`
		args = append(args, "%"+q+"%")
	}
	if t := r.URL.Query().Get("type"); t == "image" || t == "video" || t == "audio" {
		where += ` AND mime LIKE ?`
		args = append(args, t+"/%")
	}
	const per = 48
	rows, err := a.db.Query(`SELECT `+mediaCols+` FROM media WHERE `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, per+1, max(page, 0)*per)...)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		m, err := scanMedia(rows)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		items = append(items, map[string]any{"media": m, "links": a.mediaURLs(r, m)})
	}
	more := len(items) > per
	writeJSON(w, 200, map[string]any{"items": items[:min(len(items), per)], "more": more})
}

func (a *App) deleteMedia(w http.ResponseWriter, r *http.Request, u *User) {
	m, err := scanMedia(a.db.QueryRow(`SELECT `+mediaCols+` FROM media WHERE id = ?`, r.PathValue("id")))
	if err != nil || (m.UserID != u.ID && !u.IsAdmin) {
		fail(w, 404, "Introuvable")
		return
	}
	a.removeMedia(m)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// removeMedia deletes the row, and the blob once no other row points at it.
func (a *App) removeMedia(m *Media) {
	a.db.Exec(`DELETE FROM media WHERE id = ?`, m.ID)
	a.cache.Del("m:" + m.Slug)
	a.gcBlob(m.Hash)
}

func (a *App) gcBlob(hash string) {
	var n int
	a.db.QueryRow(`SELECT COUNT(*) FROM media WHERE hash = ?`, hash).Scan(&n)
	if n == 0 {
		os.Remove(a.filePath(hash))
		os.Remove(a.filePath(hash) + ".lite")
	}
}

// mediaView is what the public pages need; cached by slug.
type mediaView struct {
	Media
	Username         string
	EmbedTitle       string
	EmbedDescription string
	EmbedColor       string
	EmbedSite        string
	EmbedRaw         bool
}

func (a *App) lookup(slug string) (*mediaView, error) {
	if b, ok := a.cache.Get("m:" + slug); ok {
		v := &mediaView{}
		if json.Unmarshal(b, v) == nil {
			return v, nil
		}
	}
	v := &mediaView{}
	err := a.db.QueryRow(`SELECT m.id, m.slug, m.user_id, m.hash, m.filename, m.mime, m.size, m.lite_mime, m.lite_size, m.delete_key, m.views, m.created_at,
		u.username, u.embed_title, u.embed_description, u.embed_color, u.embed_site, u.embed_raw
		FROM media m JOIN users u ON u.id = m.user_id WHERE m.slug = ?`, slug).Scan(
		&v.ID, &v.Slug, &v.UserID, &v.Hash, &v.Filename, &v.Mime, &v.Size, &v.LiteMime, &v.LiteSize, &v.DeleteKey, &v.Views, &v.CreatedAt,
		&v.Username, &v.EmbedTitle, &v.EmbedDescription, &v.EmbedColor, &v.EmbedSite, &v.EmbedRaw)
	if err != nil {
		return nil, err
	}
	if b, err := json.Marshal(v); err == nil {
		a.cache.Set("m:"+slug, b, 10*time.Minute)
	}
	return v, nil
}

func (a *App) raw(w http.ResponseWriter, r *http.Request) {
	v, err := a.lookup(r.PathValue("slug"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.serveFile(w, r, v, strings.HasSuffix(r.URL.Path, "/lite"))
}

func (a *App) serveFile(w http.ResponseWriter, r *http.Request, v *mediaView, lite bool) {
	path, ctype := a.filePath(v.Hash), v.Mime
	if lite && v.LiteMime != "" {
		path, ctype = path+".lite", v.LiteMime
	}
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	disp := "attachment"
	if inline(ctype) {
		disp = "inline"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disp, map[string]string{"filename": v.Filename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; media-src 'self'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // content is addressed by hash
	http.ServeContent(w, r, "", time.Unix(v.CreatedAt, 0), f)
}

func isBot(ua string) bool {
	ua = strings.ToLower(ua)
	for _, b := range []string{"discordbot", "telegrambot", "twitterbot", "slackbot", "whatsapp", "facebookexternalhit", "mastodon", "matrix", "skypeuripreview", "linkedinbot", "embedly"} {
		if strings.Contains(ua, b) {
			return true
		}
	}
	return false
}

var viewTmpl = template.Must(template.New("view").Parse(`<!doctype html>
<html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta name="theme-color" content="{{.Color}}">
<meta property="og:site_name" content="{{.Site}}">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Desc}}">
<meta property="og:url" content="{{.URL}}">
{{if .IsVideo}}<meta property="og:type" content="video.other">
<meta property="og:video" content="{{.Raw}}"><meta property="og:video:secure_url" content="{{.Raw}}"><meta property="og:video:type" content="{{.Mime}}">
<meta property="og:image" content="{{.Raw}}"><meta name="twitter:card" content="player">
{{else if .IsImage}}<meta property="og:type" content="website">
<meta property="og:image" content="{{.Raw}}"><meta property="og:image:type" content="{{.Mime}}"><meta name="twitter:card" content="summary_large_image"><meta name="twitter:image" content="{{.Raw}}">
{{end}}<link rel="alternate" type="application/json+oembed" href="{{.OEmbed}}">
<style>
:root{color-scheme:dark;--bg:#0d0d12;--fg:#ececf1;--mut:#9a9aab;--card:#17171f;--acc:{{.Color}}}
body{margin:0;min-height:100vh;background:var(--bg);color:var(--fg);font:15px/1.5 system-ui,sans-serif;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:16px;padding:16px;box-sizing:border-box}
.m{max-width:min(1200px,100%);max-height:80vh;border-radius:12px;background:var(--card)}
.b{display:flex;flex-wrap:wrap;gap:8px;align-items:center;justify-content:center;color:var(--mut);font-size:14px}
a{color:var(--fg);background:var(--card);padding:8px 14px;border-radius:999px;text-decoration:none;border:1px solid #2a2a36}
a:hover{border-color:var(--acc)}
</style></head><body>
{{if .IsVideo}}<video class="m" src="{{.Src}}" controls autoplay muted loop playsinline></video>
{{else if .IsImage}}<img class="m" src="{{.Src}}" alt="{{.Filename}}">
{{else if .IsAudio}}<audio src="{{.Src}}" controls></audio>
{{end}}<div class="b"><strong style="color:var(--fg)">{{.Filename}}</strong><span>{{.SizeH}} · {{.Views}} vues</span>
<a href="{{.Raw}}">Original</a><a href="{{.Raw}}" download="{{.Filename}}">Télécharger</a>{{if .Lite}}<a href="{{.LiteURL}}">Version légère ({{.LiteH}})</a>{{end}}</div>
</body></html>`))

func expand(tmpl string, v *mediaView) string {
	return strings.NewReplacer("{filename}", v.Filename, "{size}", human(v.Size), "{user}", v.Username,
		"{date}", time.Unix(v.CreatedAt, 0).Format("02/01/2006 15:04"), "{views}", strconv.FormatInt(v.Views, 10)).Replace(tmpl)
}

func (a *App) view(w http.ResponseWriter, r *http.Request) {
	v, err := a.lookup(r.PathValue("slug"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Bots get the file itself when the owner chose "raw" embeds, and always for videos:
	// Discord only plays a video inline from a direct link (og:video needs dimensions we don't store).
	if isBot(r.UserAgent()) && (v.EmbedRaw || strings.HasPrefix(v.Mime, "video/")) {
		a.serveFile(w, r, v, false)
		return
	}
	if !isBot(r.UserAgent()) {
		a.db.Exec(`UPDATE media SET views = views + 1 WHERE id = ?`, v.ID)
	}
	b := a.base(r) + "/" + v.Slug
	saveData := r.Header.Get("Save-Data") == "on"
	src := b + "/raw"
	if saveData && v.LiteMime != "" {
		src = b + "/lite"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	viewTmpl.Execute(w, map[string]any{
		"Title": expand(v.EmbedTitle, v), "Desc": expand(v.EmbedDescription, v), "Site": v.EmbedSite, "Color": v.EmbedColor,
		"URL": b, "Raw": b + "/raw", "Src": src, "LiteURL": b + "/lite", "Lite": v.LiteMime != "", "LiteH": human(v.LiteSize),
		"Mime": v.Mime, "Filename": v.Filename, "SizeH": human(v.Size), "Views": v.Views,
		"IsVideo": strings.HasPrefix(v.Mime, "video/") || (saveData && strings.HasPrefix(v.LiteMime, "video/")),
		"IsImage": inline(v.Mime) && strings.HasPrefix(v.Mime, "image/") && !(saveData && strings.HasPrefix(v.LiteMime, "video/")),
		"IsAudio": strings.HasPrefix(v.Mime, "audio/"),
		"OEmbed":  a.base(r) + "/api/oembed?url=" + template.URLQueryEscaper(b),
	})
}

func (a *App) oembed(w http.ResponseWriter, r *http.Request) {
	slug := filepath.Base(r.URL.Query().Get("url"))
	v, err := a.lookup(slug)
	if err != nil {
		fail(w, 404, "Introuvable")
		return
	}
	writeJSON(w, 200, map[string]string{"version": "1.0", "type": "link", "title": expand(v.EmbedTitle, v),
		"author_name": expand(v.EmbedDescription, v), "provider_name": v.EmbedSite, "provider_url": a.base(r)})
}

var deleteTmpl = template.Must(template.New("del").Parse(`<!doctype html><html lang="fr"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1"><title>Supprimer {{.}}</title>
<style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#0d0d12;color:#ececf1;font:16px system-ui,sans-serif;padding:16px;box-sizing:border-box;text-align:center}
button{font:inherit;background:#e5484d;color:#fff;border:0;border-radius:999px;padding:12px 24px;cursor:pointer}</style></head>
<body><form method="post"><p>Supprimer définitivement <strong>{{.}}</strong> ?</p><button>Supprimer</button></form></body></html>`))

func (a *App) deleteConfirm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	deleteTmpl.Execute(w, r.PathValue("slug"))
}

func (a *App) deleteByKey(w http.ResponseWriter, r *http.Request) {
	m, err := scanMedia(a.db.QueryRow(`SELECT `+mediaCols+` FROM media WHERE slug = ?`, r.PathValue("slug")))
	if err != nil || subtle.ConstantTimeCompare([]byte(m.DeleteKey), []byte(r.PathValue("key"))) != 1 {
		http.NotFound(w, r)
		return
	}
	a.removeMedia(m)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width"><body style="background:#0d0d12;color:#ececf1;font:16px system-ui;display:grid;place-items:center;min-height:90vh">Fichier supprimé.</body>`)
}

func human(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d o", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %co", float64(n)/float64(div), "KMGTPE"[exp])
}
