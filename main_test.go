package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
)

func call(t *testing.T, c *http.Client, method, url string, body any) (int, map[string]any) {
	t.Helper()
	var rd *bytes.Reader
	b, _ := json.Marshal(body)
	rd = bytes.NewReader(b)
	req, _ := http.NewRequest(method, url, rd)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out := map[string]any{}
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func uploadFile(t *testing.T, c *http.Client, url, token, name string, data []byte) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", name)
	fw.Write(data)
	mw.Close()
	req, _ := http.NewRequest("POST", url+"/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out := map[string]any{}
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

var png = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{1}, 1000)...)

func TestFlow(t *testing.T) {
	app, err := newApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}

	// First account becomes admin even with registration closed; the second is refused.
	if code, out := call(t, c, "POST", srv.URL+"/api/auth/register", credentials{"admin", "password123"}); code != 200 {
		t.Fatal(code, out)
	}
	if code, _ := call(t, http.DefaultClient, "POST", srv.URL+"/api/auth/register", credentials{"bob", "password123"}); code != 403 {
		t.Fatal("registration should be closed, got", code)
	}
	_, me := call(t, c, "GET", srv.URL+"/api/me", nil)
	user := me["user"].(map[string]any)
	if user["is_admin"] != true {
		t.Fatal("first user should be admin", me)
	}
	token := user["api_token"].(string)

	// Token upload, then the same bytes again → same link, no extra row.
	code, up := uploadFile(t, http.DefaultClient, srv.URL, token, "shot.png", png)
	if code != 200 || up["media"].(map[string]any)["mime"] != "image/png" {
		t.Fatal(code, up)
	}
	_, dup := uploadFile(t, http.DefaultClient, srv.URL, token, "again.png", png)
	if dup["duplicate"] != true || dup["url"] != up["url"] {
		t.Fatal("expected duplicate", dup)
	}
	if code, _ := uploadFile(t, http.DefaultClient, srv.URL, "bad", "x.png", png); code != 401 {
		t.Fatal("bad token accepted", code)
	}

	// Public view, raw file, and bot embed.
	resp, _ := http.Get(up["raw"].(string))
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatal("raw", resp.StatusCode, resp.Header)
	}
	req, _ := http.NewRequest("GET", up["url"].(string), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Discordbot/2.0)")
	resp, _ = http.DefaultClient.Do(req)
	var page bytes.Buffer
	page.ReadFrom(resp.Body)
	if !strings.Contains(page.String(), `og:image" content="`+up["raw"].(string)) {
		t.Fatal("missing og:image", page.String())
	}

	// HTML disguised as an image must not be served inline.
	_, h := uploadFile(t, http.DefaultClient, srv.URL, token, "evil.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	resp, _ = http.Get(h["raw"].(string))
	if !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment") {
		t.Fatal("svg served inline", resp.Header)
	}

	// Quota and size limits.
	call(t, c, "PUT", srv.URL+"/api/admin/settings", map[string]int64{"default_quota": 1500, "max_upload": 5000})
	if code, out := uploadFile(t, http.DefaultClient, srv.URL, token, "b.png", append(png, 2)); code != 413 {
		t.Fatal("quota not enforced", code, out)
	}
	call(t, c, "PUT", srv.URL+"/api/admin/settings", map[string]int64{"default_quota": 0})
	if code, _ := uploadFile(t, http.DefaultClient, srv.URL, token, "big.png", make([]byte, 6000)); code != 413 {
		t.Fatal("max upload not enforced", code)
	}

	// Rate limit.
	call(t, c, "PUT", srv.URL+"/api/admin/settings", map[string]int64{"uploads_per_hour": 2})
	if code, _ := uploadFile(t, http.DefaultClient, srv.URL, token, "c.png", append(png, 3)); code != 429 {
		t.Fatal("rate limit not enforced", code)
	}

	// Delete by key removes the blob.
	slug := up["media"].(map[string]any)["slug"].(string)
	resp, _ = http.Post(up["delete_url"].(string), "", nil)
	if resp.StatusCode != 200 {
		t.Fatal("delete", resp.StatusCode)
	}
	if resp, _ = http.Get(srv.URL + "/" + slug + "/raw"); resp.StatusCode != 404 {
		t.Fatal("still served after delete")
	}

	// Admin user list (regression: nested query on the single DB connection deadlocked).
	resp, _ = c.Get(srv.URL + "/api/admin/users")
	if resp.StatusCode != 200 {
		t.Fatal("admin users", resp.StatusCode)
	}

	// Client configs embed the token.
	resp, _ = c.Get(srv.URL + "/api/clients/sharex.sxcu")
	var sx map[string]any
	json.NewDecoder(resp.Body).Decode(&sx)
	if sx["Headers"].(map[string]any)["Authorization"] != "Bearer "+token {
		t.Fatal("sxcu", sx)
	}
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		tag, cur string
		want     bool
	}{{"v1.2.0", "v1.1.9", true}, {"v1.2.0", "v1.2.0", false}, {"v1.10.0", "v1.9.0", true}, {"v0.1.0", "dev", true}, {"v1.0.0", "v2.0.0", false}} {
		if newer(c.tag, c.cur) != c.want {
			t.Errorf("newer(%s, %s) != %v", c.tag, c.cur, c.want)
		}
	}
}
