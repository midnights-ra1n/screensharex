package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (a *App) stats(w http.ResponseWriter, r *http.Request, u *User) {
	global := u.IsAdmin && r.URL.Query().Get("scope") == "server"
	where, args := "", []any{}
	if !global {
		where, args = " WHERE user_id = ?", []any{u.ID}
	}
	out := map[string]any{}

	var count, size, views int64
	a.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(size), 0), COALESCE(SUM(views), 0) FROM media`+where, args...).Scan(&count, &size, &views)
	out["count"], out["size"], out["views"] = count, size, views

	types := map[string][2]int64{}
	rows, _ := a.db.Query(`SELECT CASE WHEN mime = 'image/gif' THEN 'gif' WHEN mime LIKE 'image/%' THEN 'image' WHEN mime LIKE 'video/%' THEN 'video'
		WHEN mime LIKE 'audio/%' THEN 'audio' ELSE 'other' END t, COUNT(*), SUM(size) FROM media`+where+` GROUP BY t`, args...)
	for rows != nil && rows.Next() {
		var t string
		var c, s int64
		rows.Scan(&t, &c, &s)
		types[t] = [2]int64{c, s}
	}
	rows.Close()
	out["types"] = types

	type day struct {
		Day   string `json:"day"`
		Count int64  `json:"count"`
		Size  int64  `json:"size"`
	}
	days := []day{}
	cond := " WHERE created_at > ?"
	if !global {
		cond += " AND user_id = ?"
	}
	rows, _ = a.db.Query(`SELECT date(created_at, 'unixepoch'), COUNT(*), SUM(size) FROM media`+cond+` GROUP BY 1 ORDER BY 1`,
		append([]any{time.Now().AddDate(0, 0, -30).Unix()}, args...)...)
	for rows != nil && rows.Next() {
		var d day
		rows.Scan(&d.Day, &d.Count, &d.Size)
		days = append(days, d)
	}
	rows.Close()
	out["days"] = days

	if global {
		var users, stored, lite int64
		a.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users)
		a.db.QueryRow(`SELECT COALESCE(SUM(size), 0), COALESCE(SUM(lite_size), 0) FROM (SELECT DISTINCT hash, size, lite_size FROM media)`).Scan(&stored, &lite)
		var st syscall.Statfs_t
		syscall.Statfs(a.dataDir, &st)
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		_, ffErr := exec.LookPath("ffmpeg")
		cache := "mémoire"
		if _, ok := a.cache.(*respCache); ok {
			cache = "redis"
		}
		out["server"] = map[string]any{
			"users": users, "stored": stored + lite, "dedup_saved": size - stored,
			"disk_free": int64(st.Bavail) * int64(st.Bsize), "disk_total": int64(st.Blocks) * int64(st.Bsize),
			"uptime": int64(time.Since(a.started).Seconds()), "version": version, "go": runtime.Version(),
			"memory": mem.Sys, "goroutines": runtime.NumGoroutine(), "cache": cache, "ffmpeg": ffErr == nil,
		}
	}
	writeJSON(w, 200, out)
}

func (a *App) getSettings(w http.ResponseWriter, r *http.Request, u *User) {
	out := map[string]int64{}
	for k := range defaultSettings {
		out[k] = a.setting(k)
	}
	writeJSON(w, 200, out)
}

func (a *App) putSettings(w http.ResponseWriter, r *http.Request, u *User) {
	var in map[string]int64
	if !readJSON(w, r, &in) {
		return
	}
	for k, v := range in {
		if _, ok := defaultSettings[k]; !ok || v < 0 {
			fail(w, 400, "Paramètre invalide : "+k)
			return
		}
	}
	for k, v := range in {
		a.db.Exec(`UPDATE settings SET value = ? WHERE key = ?`, strconv.FormatInt(v, 10), k)
	}
	a.getSettings(w, r, u)
}

func (a *App) listUsers(w http.ResponseWriter, r *http.Request, u *User) {
	defQuota := a.setting("default_quota") // read before Query: the DB has a single connection
	rows, err := a.db.Query(`SELECT ` + userCols + `, (SELECT COUNT(*) FROM media WHERE user_id = users.id), (SELECT COALESCE(SUM(size), 0) FROM media WHERE user_id = users.id) FROM users ORDER BY id`)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		x := &User{}
		var count, used int64
		rows.Scan(&x.ID, &x.Username, &x.IsAdmin, &x.Quota, &x.APIToken, &x.EmbedTitle, &x.EmbedDescription, &x.EmbedColor, &x.EmbedSite, &x.EmbedRaw, &x.CreatedAt, &count, &used)
		x.APIToken = ""
		quota := x.Quota
		if quota < 0 {
			quota = defQuota
		}
		out = append(out, map[string]any{"user": x, "count": count, "used": used, "quota": quota})
	}
	writeJSON(w, 200, out)
}

type userInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IsAdmin  *bool  `json:"is_admin"`
	Quota    *int64 `json:"quota"`
}

func (a *App) createUser(w http.ResponseWriter, r *http.Request, u *User) {
	var in userInput
	if !readJSON(w, r, &in) {
		return
	}
	id, msg := a.createAccount(in.Username, in.Password, in.IsAdmin != nil && *in.IsAdmin)
	if msg != "" {
		fail(w, 400, msg)
		return
	}
	if in.Quota != nil {
		a.db.Exec(`UPDATE users SET quota = ? WHERE id = ?`, max(*in.Quota, -1), id)
	}
	writeJSON(w, 200, map[string]int64{"id": id})
}

func (a *App) updateUser(w http.ResponseWriter, r *http.Request, u *User) {
	var in userInput
	if !readJSON(w, r, &in) {
		return
	}
	id := r.PathValue("id")
	if in.IsAdmin != nil {
		if id == strconv.FormatInt(u.ID, 10) && !*in.IsAdmin {
			fail(w, 400, "Vous ne pouvez pas retirer vos propres droits")
			return
		}
		a.db.Exec(`UPDATE users SET is_admin = ? WHERE id = ?`, *in.IsAdmin, id)
	}
	if in.Quota != nil {
		a.db.Exec(`UPDATE users SET quota = ? WHERE id = ?`, max(*in.Quota, -1), id)
	}
	if in.Password != "" {
		if len(in.Password) < 8 || len(in.Password) > 72 {
			fail(w, 400, "Mot de passe : 8 à 72 caractères")
			return
		}
		h, _ := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		a.db.Exec(`UPDATE users SET password = ? WHERE id = ?`, h, id)
		a.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, id)
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *App) deleteUser(w http.ResponseWriter, r *http.Request, u *User) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if id == u.ID {
		fail(w, 400, "Vous ne pouvez pas supprimer votre propre compte")
		return
	}
	rows, err := a.db.Query(`SELECT `+mediaCols+` FROM media WHERE user_id = ?`, id)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	var media []*Media
	for rows.Next() {
		if m, err := scanMedia(rows); err == nil {
			media = append(media, m)
		}
	}
	rows.Close()
	for _, m := range media {
		a.removeMedia(m)
	}
	a.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// export streams a .tar.gz holding a consistent DB snapshot and every stored file.
func (a *App) export(w http.ResponseWriter, r *http.Request, u *User) {
	snap := filepath.Join(a.dataDir, "export-snapshot.db")
	os.Remove(snap)
	if _, err := a.db.Exec(`VACUUM INTO ?`, snap); err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer os.Remove(snap)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="screensharex-%s.tar.gz"`, time.Now().Format("2006-01-02")))
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	add := func(name, src string) error {
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		st, _ := f.Stat()
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o640, Size: st.Size(), ModTime: st.ModTime()}); err != nil {
			return err
		}
		_, err = io.Copy(tw, f)
		return err
	}
	err := add("screensharex.db", snap)
	files := filepath.Join(a.dataDir, "files")
	if err == nil {
		err = filepath.WalkDir(files, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.HasPrefix(d.Name(), "upload-") || strings.HasSuffix(d.Name(), ".tmp") {
				return err
			}
			rel, _ := filepath.Rel(files, p)
			return add("files/"+filepath.ToSlash(rel), p)
		})
	}
	if err != nil {
		log.Printf("export: %v", err) // headers are already sent; the truncated archive will fail to extract
		return
	}
	tw.Close()
	gz.Close()
}

var blobName = regexp.MustCompile(`^files/[0-9a-f]{2}/[0-9a-f]{64}(\.lite)?$`)

// importArchive restores an export: the DB is replaced, files are merged, then the server restarts.
func (a *App) importArchive(w http.ResponseWriter, r *http.Request, u *User) {
	gz, err := gzip.NewReader(r.Body)
	if err != nil {
		fail(w, 400, "Archive .tar.gz invalide")
		return
	}
	staged := filepath.Join(a.dataDir, "import.db")
	defer os.Remove(staged)
	gotDB := false
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fail(w, 400, "Archive corrompue : "+err.Error())
			return
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Clean(h.Name)
		var dst string
		switch {
		case name == "screensharex.db":
			dst, gotDB = staged, true
		case blobName.MatchString(name):
			dst = filepath.Join(a.dataDir, filepath.FromSlash(name))
			if _, err := os.Stat(dst); err == nil {
				continue // content-addressed: same name, same bytes
			}
		default:
			continue
		}
		if err := writeFile(dst, tr); err != nil {
			fail(w, 500, err.Error())
			return
		}
	}
	if !gotDB {
		fail(w, 400, "L'archive ne contient pas de base de données")
		return
	}
	check, err := openDB(staged) // validates and migrates the imported DB
	if err != nil {
		fail(w, 400, "Base importée invalide : "+err.Error())
		return
	}
	check.Close()
	a.db.Close()
	live := filepath.Join(a.dataDir, "screensharex.db")
	os.Remove(live + "-wal")
	os.Remove(live + "-shm")
	if err := os.Rename(staged, live); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"message": "Import terminé, redémarrage du serveur…"})
	go restart()
}

func writeFile(dst string, r io.Reader) error {
	os.MkdirAll(filepath.Dir(dst), 0o750)
	f, err := os.Create(dst + ".tmp")
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(dst + ".tmp")
		return err
	}
	return os.Rename(dst+".tmp", dst)
}

// restart replaces the process with a fresh copy of the (possibly updated) binary.
func restart() {
	time.Sleep(500 * time.Millisecond) // let the response flush
	exe, err := os.Executable()
	if err == nil {
		err = syscall.Exec(exe, os.Args, os.Environ())
	}
	log.Fatalf("restart: %v", err)
}

const debUpdater = "/usr/lib/screensharex/update.sh"

type release struct {
	Tag  string `json:"tag_name"`
	Body string `json:"body"`
	URL  string `json:"html_url"`
	Date string `json:"published_at"`
}

func newer(tag, cur string) bool {
	parse := func(s string) (v [3]int) {
		for i, p := range strings.SplitN(strings.TrimPrefix(s, "v"), ".", 3) {
			v[i], _ = strconv.Atoi(strings.SplitN(p, "-", 2)[0])
		}
		return
	}
	t, c := parse(tag), parse(cur)
	for i := range t {
		if t[i] != c[i] {
			return t[i] > c[i]
		}
	}
	return false
}

func installMode() string {
	if _, err := os.Stat(debUpdater); err == nil {
		return "deb"
	}
	for _, f := range []string{"/.dockerenv", "/run/.containerenv"} { // Docker, Podman
		if _, err := os.Stat(f); err == nil {
			return "docker"
		}
	}
	return "manual"
}

func (a *App) checkUpdate(w http.ResponseWriter, r *http.Request, u *User) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		fail(w, 502, "GitHub injoignable : "+err.Error())
		return
	}
	defer resp.Body.Close()
	var rel release
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&rel) != nil {
		fail(w, 502, "Aucune release trouvée sur github.com/"+repo)
		return
	}
	writeJSON(w, 200, map[string]any{"current": version, "latest": rel, "available": newer(rel.Tag, version), "mode": installMode()})
}

var tagRe = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// installUpdate asks the root-side updater (a systemd path unit shipped in the .deb) to install
// an official release. Only the tag is passed: the updater downloads and verifies the package itself.
func (a *App) installUpdate(w http.ResponseWriter, r *http.Request, u *User) {
	var in struct {
		Tag string `json:"tag"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if installMode() != "deb" {
		fail(w, 400, "Mise à jour automatique disponible uniquement avec le paquet .deb")
		return
	}
	if !tagRe.MatchString(in.Tag) {
		fail(w, 400, "Version invalide")
		return
	}
	if err := os.WriteFile(filepath.Join(a.dataDir, "update-request"), []byte(in.Tag), 0o640); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"message": "Installation de " + in.Tag + " en cours, le serveur va redémarrer…"})
}
