package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// makeLite builds a lightweight variant (WebP for pictures, small H.264 MP4 for GIFs and videos)
// for viewers on slow or metered connections. Needs ffmpeg; silently skipped without it.
func (a *App) makeLite(hash, ctype string, size int64) {
	if size < 256<<10 {
		return // already small
	}
	var args []string
	var liteMime string
	switch {
	case ctype == "image/gif" || strings.HasPrefix(ctype, "video/"):
		liteMime = "video/mp4"
		args = []string{"-map", "0:v:0", "-map", "0:a:0?", "-vf", "scale='min(854,iw)':-2", "-c:v", "libx264", "-preset", "veryfast",
			"-crf", "30", "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "64k", "-movflags", "+faststart", "-f", "mp4"}
	case strings.HasPrefix(ctype, "image/") && inline(ctype):
		liteMime = "image/webp"
		args = []string{"-vf", "scale='min(1600,iw)':-1", "-frames:v", "1", "-c:v", "libwebp", "-quality", "70", "-f", "webp"}
	default:
		return
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return
	}
	a.litesem <- struct{}{}
	defer func() { <-a.litesem }()

	src := a.filePath(hash)
	tmp := src + ".lite.tmp"
	defer os.Remove(tmp)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", append(append([]string{"-hide_banner", "-loglevel", "error", "-y", "-i", src}, args...), tmp)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("lite %s: %v %s", hash[:12], err, out)
		return
	}
	st, err := os.Stat(tmp)
	if err != nil || st.Size() >= size {
		return // not smaller, keep only the original
	}
	if os.Rename(tmp, src+".lite") != nil {
		return
	}
	a.db.Exec(`UPDATE media SET lite_mime = ?, lite_size = ? WHERE hash = ?`, liteMime, st.Size(), hash)
	rows, err := a.db.Query(`SELECT slug FROM media WHERE hash = ?`, hash)
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
