<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/logo-dark.svg">
  <img src="docs/logo-light.svg" width="352" alt="ScreenShareX">
</picture>

**Self-hosted screenshot, GIF and video hosting — a modern XBackBone alternative.**

One Go binary · SQLite · embedded TypeScript UI · works with ShareX and friends

[![CI](https://github.com/midnights-ra1n/screensharex/actions/workflows/ci.yml/badge.svg)](https://github.com/midnights-ra1n/screensharex/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/midnights-ra1n/screensharex?sort=semver)](https://github.com/midnights-ra1n/screensharex/releases)
[![Docker](https://img.shields.io/badge/ghcr.io-screensharex-2496ED?logo=docker&logoColor=white)](https://github.com/midnights-ra1n/screensharex/pkgs/container/screensharex)
![Go](https://img.shields.io/badge/Go-backend-00ADD8?logo=go&logoColor=white)
![TypeScript](https://img.shields.io/badge/TypeScript-frontend-3178C6?logo=typescript&logoColor=white)
![SQLite](https://img.shields.io/badge/SQLite-database-003B57?logo=sqlite&logoColor=white)

[Features](#-features) · [Install](#-installation) · [Configuration](#%EF%B8%8F-configuration) · [Capture tools](#-capture-tools) · [API](#-api-reference) · [Development](#-development)

</div>

---

## ✨ Features

| | |
|---|---|
| 🖼️ **Modern web UI** | Mobile-first gallery with search and type filters, drag-and-drop uploads, per-user and server-wide statistics with charts. |
| 📤 **Uploads from anywhere** | Web UI, ShareX, XerahS, ishare, ScreenCloud, Spectacle (KDE), the macOS/iOS share sheet, a CLI script, or the plain HTTP API. |
| 👥 **Multi-user** | Admin and regular accounts. Registration can be open or closed; the first account is always the admin. |
| 📦 **Quotas and limits** | Per-user storage quota, max file size and uploads per hour, set separately for the web UI and for API clients. |
| 🧬 **Deduplication** | Files are stored once, by SHA-256. Uploading the same file twice returns your existing link. |
| 🪶 **Lite versions** | With ffmpeg, images get a WebP copy and GIFs and videos a small H.264 MP4, for slow or metered connections (`Save-Data`). |
| 💬 **Rich embeds** | Custom Discord/Telegram/Slack/Mastodon embeds (title, description, color, site name, placeholders) or "raw" mode, which sends the file itself so it plays inside the app. |
| ⚡ **Fast cache** | In-memory by default, or any Redis-protocol server: Valkey, KeyDB, Dragonfly, Redis. |
| 💾 **Backup and restore** | One-click export of the database and every file to a `.tar.gz`, and one-click import. |
| 🔄 **Built-in updates** | On Debian/Ubuntu, check for, install and restart into a new release from the admin page. |
| 🐳 **Easy to deploy** | `.deb` package for Debian/Ubuntu and a Docker image (x86_64). |

---

## 🚀 Installation

### Debian / Ubuntu (`.deb`)

Download the latest `screensharex_X.Y.Z_amd64.deb` from the [releases page](https://github.com/midnights-ra1n/screensharex/releases), then:

```sh
sudo apt install ./screensharex_X.Y.Z_amd64.deb
sudo apt install ffmpeg          # optional, enables lite versions
```

| | |
|---|---|
| Service | `systemctl status screensharex` |
| Listens on | `:8080` |
| Data (DB and files) | `/var/lib/screensharex` |
| Configuration | `/etc/default/screensharex`, then `systemctl restart screensharex` |
| Updates | **Admin → Updates** in the web UI |

### Docker

```sh
curl -O https://raw.githubusercontent.com/midnights-ra1n/screensharex/dev/docker-compose.yml
docker compose up -d
```

The bundled [`docker-compose.yml`](docker-compose.yml) runs ScreenShareX with a small memory-only Valkey cache. ffmpeg is already in the image.

Or with plain Docker:

```sh
docker run -d --name screensharex -p 8080:8080 -v screensharex-data:/data \
  -e SSX_BASE_URL=https://img.example.com \
  ghcr.io/midnights-ra1n/screensharex:latest
```

To update: `docker compose pull && docker compose up -d`.

### First start

Open `http://your-server:8080`. **The first account you create becomes the administrator**, even when registration is closed, so create it right after installing.

### Behind a reverse proxy

Set `SSX_BASE_URL` to the public URL so generated links are correct, and allow large request bodies.

<details>
<summary><b>nginx</b></summary>

```nginx
server {
    server_name img.example.com;
    client_max_body_size 200M;   # at least your max upload size

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_request_buffering off;
    }
}
```
</details>

<details>
<summary><b>Caddy</b></summary>

```caddy
img.example.com {
    reverse_proxy 127.0.0.1:8080
}
```
</details>

---

## ⚙️ Configuration

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `SSX_ADDR` | `:8080` | Listen address. |
| `SSX_DATA` | `./data` (`/data` in Docker) | Folder for the SQLite DB and the files. |
| `SSX_BASE_URL` | taken from the request | Public URL used in links, for example `https://img.example.com`. Recommended behind a reverse proxy. |
| `SSX_REDIS` | in-memory cache | `redis://[:password@]host:6379/0`, for Valkey, KeyDB, Dragonfly or Redis. |
| `SSX_WEB_DIR` | embedded UI | Development only: serve the UI from disk. |

### Server settings (Admin → Settings)

These are changed live from the admin page, or with [`PUT /api/admin/settings`](#put-apiadminsettings).

| Key | Default | Description |
|---|---|---|
| `registration` | `0` | `1` lets anyone create an account. |
| `default_quota` | `1073741824` (1 GiB) | Storage per user in bytes. `0` means unlimited. Each user can have their own quota. |
| `max_upload` | `104857600` (100 MiB) | Max file size from the web UI. |
| `max_upload_api` | `104857600` (100 MiB) | Max file size from capture tools and API clients. |
| `uploads_per_hour` | `120` | Uploads per user per hour from the web UI. `0` means unlimited. |
| `uploads_per_hour_api` | `120` | Uploads per user per hour from API clients. `0` means unlimited. |
| `compress` | `1` | `1` builds lite versions with ffmpeg. |

---

## 🧠 How it works

<details open>
<summary><b>Storage and deduplication</b></summary>

Each upload is hashed with SHA-256 and stored at `files/<first 2 chars>/<hash>`. Identical files uploaded by different users share one file on disk. Each user still gets their own link, and the file is only removed when the last link to it is deleted. If you upload a file you already have, the server returns your existing link with `"duplicate": true`.
</details>

<details>
<summary><b>Lite versions</b></summary>

When `compress` is on and `ffmpeg` is installed, files of 256 KiB or more get a lighter copy in the background, one job at a time:

| Source | Lite version |
|---|---|
| Images (PNG, JPEG, WebP…) | WebP, at most 1600 px wide, quality 70 |
| GIFs and videos | H.264 MP4, at most 854 px wide, CRF 30, AAC 64 kbit/s, fast start |

The copy is only kept if it is smaller than the original. It is served at `/<slug>/lite`, and the viewer page uses it when the browser sends `Save-Data: on`.
</details>

<details>
<summary><b>Embeds</b></summary>

Each user picks the title, description, accent color and site name of their embeds in **Settings**. Title and description accept these placeholders:

| Placeholder | Example |
|---|---|
| `{filename}` | `capture.png` |
| `{size}` | `1.2 Mo` |
| `{user}` | `alice` |
| `{date}` | `23/10/2026 14:05` |
| `{views}` | `42` |

Chat app bots (Discord, Telegram, Slack, WhatsApp, Mastodon, Matrix, X/Twitter, Facebook, LinkedIn, Skype, Embedly) get OpenGraph tags and an oEmbed link. In **raw** mode, and always for videos, bots get the file itself so it shows and plays inside the app. Bot visits do not count as views.
</details>

<details>
<summary><b>Backup and restore</b></summary>

**Admin → Export** downloads `screensharex-YYYY-MM-DD.tar.gz`, which holds a consistent snapshot of `screensharex.db` and the whole `files/` folder. **Import** replaces the database, adds the files that are missing, and restarts the server.

> ⚠️ Import replaces **every** account, setting and link with the ones in the archive.
</details>

<details>
<summary><b>Updates</b></summary>

**Admin → Updates** checks the latest GitHub release. With the `.deb` package, clicking **Install** asks a root systemd helper (`screensharex-update.path`) to download that release's `.deb`, check it against `SHA256SUMS`, install it and restart the service. The web process never runs anything as root. With Docker, pull the new image instead.
</details>

---

## 📸 Capture tools

Ready-made config files, with your URL and API token filled in, are in **Integrations** in the web UI (or at [`GET /api/clients/{name}`](#get-apiclientsname)).

| Tool | Platform | Setup |
|---|---|---|
| **ShareX** | Windows | Download `sharex.sxcu` and double-click it. In *Destinations*, pick *Custom image/file uploader → ScreenShareX*. |
| **XerahS** | Windows · macOS · Linux | Import `xerahs.sxcu` in *Destinations → Custom uploaders*. |
| **ishare** | macOS | Open `ishare.iscu`, then pick it as the active uploader. |
| **Spectacle** | KDE Plasma | Install the script and desktop entry (below), then *Export → Open with → ScreenShareX*. |
| **ScreenCloud** | Windows · macOS · Linux | Install the script, turn on the *Shell Script* service, command: `~/.local/bin/screensharex-upload {s}`. |
| **macOS / iOS share sheet** | macOS · iOS | A Shortcut with *Get contents of URL* (see below). |
| **CLI** | Linux · macOS · WSL | `screensharex-upload capture.png clip.mp4` |
| **Anything else** | — | Use the [HTTP API](#post-apiupload). |

<details>
<summary><b>CLI script (Spectacle, ScreenCloud, cron…)</b></summary>

```sh
curl -H "Authorization: Bearer $TOKEN" -o screensharex-upload https://img.example.com/api/clients/screensharex-upload
curl -H "Authorization: Bearer $TOKEN" -o screensharex.desktop https://img.example.com/api/clients/screensharex.desktop
install -Dm755 screensharex-upload  ~/.local/bin/screensharex-upload
install -Dm644 screensharex.desktop ~/.local/share/applications/screensharex.desktop
```

The script uploads each file, prints the link, copies it to the clipboard (`wl-copy`, `xclip`, `xsel` or `pbcopy`) and shows a notification. `SSX_URL` and `SSX_TOKEN` in the environment override the values built into the script.
</details>

<details>
<summary><b>macOS / iOS Shortcut</b></summary>

1. In **Shortcuts**, create a shortcut and turn on **Show in Share Sheet** (images, media, files).
2. Add **Get Contents of URL**:
   - URL: `https://img.example.com/api/upload`
   - Method: `POST`
   - Header: `Authorization: Bearer <your token>`
   - Request body: **Form**, with a **File** field named `file` set to *Shortcut Input*.
3. Add **Get Dictionary Value** `url`, then **Copy to Clipboard**.
</details>

---

## 📚 API reference

### Basics

- **Base URL:** your server, for example `https://img.example.com`. All endpoints below are relative to it.
- **Format:** JSON in and out, except uploads (`multipart/form-data`), import (raw `.tar.gz`) and file downloads.
- **Errors:** a non-2xx status with `{"error": "message"}`. Messages are currently in French.
- **Sizes** are in bytes. **Dates** are Unix timestamps in seconds.

### Authentication

Two ways to authenticate:

| Method | Used by | How |
|---|---|---|
| **Session cookie** | Web UI | `ssx_session`, set by `/api/auth/login` and `/api/auth/register`, valid 30 days, `HttpOnly`, `SameSite=Lax`. |
| **API token** | Capture tools, scripts | Any one of: `Authorization: Bearer <token>`, `X-API-Key: <token>`, or a `token` form or query field (for tools that can only send form fields). |

Find your token in **Settings** or with `GET /api/me`. Regenerate it with `POST /api/me/token`; the old one stops working at once.

> **Web vs API limits:** requests made with an API token use `max_upload_api` and `uploads_per_hour_api`. Requests made with a session cookie use `max_upload` and `uploads_per_hour`. If a request carries both, the cookie wins.

### Status codes

| Code | Meaning |
|---|---|
| `200` | Success |
| `400` | Invalid input (bad JSON, failed validation) |
| `401` | Not logged in, or invalid token or credentials |
| `403` | Admin only, or registration closed |
| `404` | Not found, or not yours |
| `413` | File too large, or quota exceeded |
| `415` | File type not allowed (only images, GIFs, videos and audio) |
| `429` | Too many uploads this hour, or too many failed logins |
| `502` | GitHub unreachable (update check) |

### Endpoint overview

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | [`/api/public`](#get-apipublic) | — | Server version and registration status |
| `POST` | [`/api/auth/register`](#post-apiauthregister) | — | Create an account |
| `POST` | [`/api/auth/login`](#post-apiauthlogin) | — | Log in |
| `POST` | [`/api/auth/logout`](#post-apiauthlogout) | — | Log out |
| `POST` | [`/api/upload`](#post-apiupload) | 🔑 user | Upload a file |
| `GET` | [`/api/me`](#get-apime) | 🔑 user | Current user, usage, limits |
| `PATCH` | [`/api/me`](#patch-apime) | 🔑 user | Update embed settings or password |
| `POST` | [`/api/me/token`](#post-apimetoken) | 🔑 user | Regenerate the API token |
| `GET` | [`/api/media`](#get-apimedia) | 🔑 user | List your uploads |
| `DELETE` | [`/api/media/{id}`](#delete-apimediaid) | 🔑 user | Delete an upload |
| `GET` | [`/api/stats`](#get-apistats) | 🔑 user | Statistics |
| `GET` | [`/api/clients/{name}`](#get-apiclientsname) | 🔑 user | Download a capture tool config |
| `GET` | [`/api/oembed`](#get-apioembed) | — | oEmbed data for a link |
| `GET` `PUT` | [`/api/admin/settings`](#get-apiadminsettings) | 👑 admin | Server settings |
| `GET` `POST` | [`/api/admin/users`](#get-apiadminusers) | 👑 admin | List or create users |
| `PATCH` `DELETE` | [`/api/admin/users/{id}`](#patch-apiadminusersid) | 👑 admin | Update or delete a user |
| `GET` | [`/api/admin/export`](#get-apiadminexport) | 👑 admin | Download a full backup |
| `POST` | [`/api/admin/import`](#post-apiadminimport) | 👑 admin | Restore a backup |
| `GET` `POST` | [`/api/admin/update`](#get-apiadminupdate) | 👑 admin | Check for or install updates |
| `GET` | [`/{slug}`](#public-media-urls) | — | Viewer page |
| `GET` | [`/{slug}/raw`](#public-media-urls) | — | Original file |
| `GET` | [`/{slug}/lite`](#public-media-urls) | — | Lite version (or the original) |
| `GET` `POST` `DELETE` | [`/{slug}/delete/{key}`](#public-media-urls) | delete key | Delete without logging in |

### Objects

<details open>
<summary><b>Media</b></summary>

```json
{
  "id": 42,
  "slug": "k7m2xq9p",
  "user_id": 1,
  "hash": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
  "filename": "capture.png",
  "mime": "image/png",
  "size": 482113,
  "lite_mime": "image/webp",
  "lite_size": 61204,
  "views": 3,
  "created_at": 1792761600
}
```

| Field | Description |
|---|---|
| `slug` | 8-character public ID used in links (no look-alike characters). |
| `hash` | SHA-256 of the file. |
| `mime` | Detected from the file content, then from the extension. |
| `lite_mime`, `lite_size` | Lite version, or `""` and `0` if there is none (yet). |
| `views` | Viewer page visits, not counting bots. |
</details>

<details open>
<summary><b>Links</b></summary>

```json
{
  "url": "https://img.example.com/k7m2xq9p",
  "raw": "https://img.example.com/k7m2xq9p/raw",
  "thumb": "https://img.example.com/k7m2xq9p/lite",
  "delete_url": "https://img.example.com/k7m2xq9p/delete/ABCDEFGHIJKLMNOPQRST"
}
```
</details>

<details>
<summary><b>User</b></summary>

```json
{
  "id": 1,
  "username": "alice",
  "is_admin": true,
  "quota": -1,
  "api_token": "JBSWY3DPEHPK3PXPJBSWY3DPEH",
  "embed_title": "{filename}",
  "embed_description": "{size} • {date}",
  "embed_color": "#7c5cff",
  "embed_site": "ScreenShareX",
  "embed_raw": false,
  "created_at": 1792761600
}
```

`quota` is in bytes; `-1` means the server default. `api_token` is only included for your own account.
</details>

---

### Public

#### `GET /api/public`

Server info for the login page.

```json
{ "version": "v1.2.0", "repo": "midnights-ra1n/screensharex", "registration": false, "setup": false }
```

`setup` is `true` while no account exists. `registration` is then also `true`.

#### `GET /api/oembed`

| Query | Description |
|---|---|
| `url` | A media link, for example `https://img.example.com/k7m2xq9p` |

```json
{
  "version": "1.0", "type": "link",
  "title": "capture.png", "author_name": "471 Ko • 23/10/2026 14:05",
  "provider_name": "ScreenShareX", "provider_url": "https://img.example.com"
}
```

---

### Authentication

#### `POST /api/auth/register`

```json
{ "username": "alice", "password": "correct horse battery" }
```

- `username`: 3 to 32 characters, letters, digits, `_` `.` `-`. Not case-sensitive and unique.
- `password`: 8 to 72 characters.

Returns `{"ok": true}` and sets the session cookie. Returns `403` when registration is closed, except for the first account, which becomes admin.

#### `POST /api/auth/login`

Same body as register. Returns `{"ok": true}` and sets the session cookie.

After 10 failed attempts from the same IP within 15 minutes, returns `429`.

> Behind a reverse proxy, every client has the proxy's IP, so they share this limit.

#### `POST /api/auth/logout`

Ends the current session. Returns `{"ok": true}`.

---

### Uploads

#### `POST /api/upload`

Upload one file as `multipart/form-data`. The field should be named `file`; if it is not, the first file field is used.

```sh
curl -H "Authorization: Bearer $TOKEN" -F "file=@capture.png" https://img.example.com/api/upload
```

```json
{
  "url": "https://img.example.com/k7m2xq9p",
  "raw": "https://img.example.com/k7m2xq9p/raw",
  "thumb": "https://img.example.com/k7m2xq9p/lite",
  "delete_url": "https://img.example.com/k7m2xq9p/delete/ABCDEFGHIJKLMNOPQRST",
  "media": { "id": 42, "slug": "k7m2xq9p", "...": "..." }
}
```

The links are at the top level so capture tools can read them easily (`{json:url}` in ShareX). If you already uploaded the same file, the response is the same and includes `"duplicate": true`.

| Error | Cause |
|---|---|
| `400` | No file in the request |
| `401` | Missing or invalid token |
| `413` | File over the size limit, or quota exceeded |
| `415` | Not an image, GIF, video or audio file (SVG, archives and documents are refused) |
| `429` | Hourly upload limit reached |

<details>
<summary>More examples</summary>

```sh
# Token as a form field
curl -F "token=$TOKEN" -F "file=@clip.mp4" https://img.example.com/api/upload

# Just print the link
curl -s -H "X-API-Key: $TOKEN" -F "file=@capture.png" https://img.example.com/api/upload | jq -r .url
```

```python
import requests
r = requests.post("https://img.example.com/api/upload",
                  headers={"Authorization": f"Bearer {TOKEN}"},
                  files={"file": open("capture.png", "rb")})
r.raise_for_status()
print(r.json()["url"])
```

```ts
const form = new FormData();
form.append("file", file);
const res = await fetch("https://img.example.com/api/upload", {
  method: "POST",
  headers: { Authorization: `Bearer ${token}` },
  body: form,
});
const { url, error } = await res.json();
```
</details>

---

### Account

#### `GET /api/me`

```json
{
  "user": { "id": 1, "username": "alice", "api_token": "…", "...": "..." },
  "used": 52428800,
  "count": 128,
  "quota": 1073741824,
  "max_upload": 104857600,
  "base_url": "https://img.example.com"
}
```

`quota` is the quota that applies to you (`0` = unlimited). `max_upload` is the web UI limit.

#### `PATCH /api/me`

Does one of two things, depending on the body.

**Update embed settings.** Send every embed field: missing fields are reset or fail validation.

```json
{
  "embed_title": "{filename}",
  "embed_description": "Uploaded by {user} · {views} views",
  "embed_color": "#ff5c8a",
  "embed_site": "My screenshots",
  "embed_raw": false
}
```

Limits: `embed_color` must be `#RRGGBB`; title up to 200 characters, description up to 500, site up to 100.

**Change password.** Logs out every other session and starts a new one here.

```json
{ "current_password": "old password", "password": "new password" }
```

Both return `{"ok": true}`.

#### `POST /api/me/token`

Creates a new API token. The old one stops working at once.

```json
{ "api_token": "NEWTOKENNEWTOKENNEWTOKEN12" }
```

> Config files you already downloaded for ShareX and the other tools contain the old token, so download them again.

---

### Media

#### `GET /api/media`

Lists your uploads, newest first, 48 per page.

| Query | Description |
|---|---|
| `page` | Page number, starting at `0` |
| `q` | Text the filename must contain |
| `type` | `image`, `video` or `audio` |

```json
{
  "items": [
    { "media": { "id": 42, "slug": "k7m2xq9p", "...": "..." },
      "links": { "url": "…", "raw": "…", "thumb": "…", "delete_url": "…" } }
  ],
  "more": true
}
```

`more` is `true` when there is another page.

#### `DELETE /api/media/{id}`

Deletes one of your uploads. Admins can delete anyone's. Returns `{"ok": true}`, or `404` if it does not exist or is not yours.

---

### Statistics

#### `GET /api/stats`

| Query | Description |
|---|---|
| `scope=server` | Admin only: stats for the whole server instead of just you |

```json
{
  "count": 128,
  "size": 52428800,
  "views": 940,
  "types": { "image": [100, 31457280], "gif": [12, 8388608], "video": [16, 12582912] },
  "days": [ { "day": "2026-10-22", "count": 5, "size": 2097152 } ],
  "server": {
    "users": 4, "stored": 41943040, "dedup_saved": 10485760,
    "disk_free": 85899345920, "disk_total": 107374182400,
    "uptime": 86400, "version": "v1.2.0", "go": "go1.26.0",
    "memory": 25165824, "goroutines": 12, "cache": "redis", "ffmpeg": true
  }
}
```

| Field | Description |
|---|---|
| `types` | `[count, bytes]` for each of `image`, `gif`, `video`, `audio`, `other` |
| `days` | Uploads per day (UTC) over the last 30 days |
| `server` | Only with `scope=server` |
| `server.stored` | Real disk usage, originals plus lite versions, each file counted once |
| `server.dedup_saved` | Bytes saved by deduplication |
| `server.cache` | `"redis"` or `"mémoire"` (memory) |

---

### Capture tool configs

#### `GET /api/clients/{name}`

Downloads a ready-to-use file with your URL and token filled in.

| `name` | Content |
|---|---|
| `sharex.sxcu` | ShareX custom uploader |
| `xerahs.sxcu` | XerahS (same format as ShareX) |
| `ishare.iscu` | ishare custom uploader |
| `screensharex-upload` | Bash upload script |
| `screensharex.desktop` | Linux desktop entry ("Open with" menu) |

---

### Admin

All admin endpoints need an admin account and return `403` otherwise.

#### `GET /api/admin/settings`

```json
{
  "registration": 0, "default_quota": 1073741824,
  "max_upload": 104857600, "max_upload_api": 104857600,
  "uploads_per_hour": 120, "uploads_per_hour_api": 120, "compress": 1
}
```

#### `PUT /api/admin/settings`

Send only the keys you want to change. Values must be integers of `0` or more. See [Server settings](#server-settings-admin--settings). Returns all settings.

```sh
curl -X PUT -H "Authorization: Bearer $TOKEN" \
  -d '{"registration": 1, "max_upload_api": 524288000}' \
  https://img.example.com/api/admin/settings
```

#### `GET /api/admin/users`

```json
[
  { "user": { "id": 1, "username": "alice", "is_admin": true, "quota": -1, "...": "..." },
    "count": 128, "used": 52428800, "quota": 1073741824 }
]
```

`user.quota` is what is set on the account (`-1` = default). The top-level `quota` is what applies.

#### `POST /api/admin/users`

```json
{ "username": "bob", "password": "at least 8 chars", "is_admin": false, "quota": 5368709120 }
```

`is_admin` and `quota` are optional. Returns `{"id": 2}`.

#### `PATCH /api/admin/users/{id}`

Every field is optional:

```json
{ "is_admin": true, "quota": -1, "password": "new password" }
```

- `quota`: bytes; `-1` = server default; `0` = unlimited.
- `password`: also logs the user out everywhere.
- You cannot remove your own admin rights.

Returns `{"ok": true}`.

#### `DELETE /api/admin/users/{id}`

Deletes the user and all of their uploads. You cannot delete yourself. Returns `{"ok": true}`.

#### `GET /api/admin/export`

Downloads a `.tar.gz` with `screensharex.db` and `files/…`.

```sh
curl -H "Authorization: Bearer $TOKEN" -o backup.tar.gz https://img.example.com/api/admin/export
```

#### `POST /api/admin/import`

Send the archive as the **raw request body**, not as multipart:

```sh
curl -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/gzip" \
  --data-binary @backup.tar.gz https://img.example.com/api/admin/import
```

The database is checked before it replaces the current one. Files that already exist are skipped. The server answers `{"message": "…"}`, then restarts.

> ⚠️ Every account, setting and link is replaced with the archive's. Your token may change.

#### `GET /api/admin/update`

```json
{
  "current": "v1.1.0",
  "latest": { "tag_name": "v1.2.0", "body": "release notes…", "html_url": "https://github.com/…", "published_at": "2026-10-20T12:00:00Z" },
  "available": true,
  "mode": "deb"
}
```

`mode` is `deb` (can update from the UI), `docker`, or `manual`.

#### `POST /api/admin/update`

`.deb` installs only.

```json
{ "tag": "v1.2.0" }
```

Returns `{"message": "…"}`. The package is downloaded from GitHub releases, checked against `SHA256SUMS`, installed, and the service restarts.

---

### Public media URLs

| URL | Description |
|---|---|
| `GET /{slug}` | Viewer page with embed tags. Chat app bots get the file itself in raw mode and for videos. Browsers sending `Save-Data: on` get the lite version. |
| `GET /{slug}/raw` | Original file. Cached forever (`immutable`). Images, videos and audio are shown in the browser (except SVG); everything else downloads. |
| `GET /{slug}/lite` | Lite version, or the original if there is none. |
| `GET /{slug}/delete/{key}` | Confirmation page. |
| `POST` or `DELETE /{slug}/delete/{key}` | Deletes the file without logging in. This is what ShareX and ishare call. |

---

## 🛠️ Development

**Requirements:** Go (see `go.mod`), Node.js 22, and optionally ffmpeg.

```sh
make dev                      # server and UI on :8080; the UI rebuilds on save (refresh the page)
make test                     # build the UI, go vet, go test
make build                    # dist/screensharex-linux-amd64 (static, CGO-free)
make deb VERSION=v0.1.0       # dist/screensharex_0.1.0_amd64.deb
docker build -t screensharex .
```

### Project layout

```
.
├── main.go          # entry point, routes, config
├── auth.go          # accounts, sessions, tokens, login rate limit
├── media.go         # upload, dedup, viewer page, embeds, oEmbed, delete links
├── lite.go          # ffmpeg lite versions
├── admin.go         # stats, settings, users, export/import, updates
├── clients.go       # ShareX / ishare / script config files
├── cache.go         # in-memory and Redis-protocol cache
├── db.go            # SQLite schema and settings
├── web/             # Preact + TypeScript + Vite UI, embedded in the binary
└── packaging/       # .deb build, systemd units, root-side updater
```

### Releasing

Push a `vX.Y.Z` tag. CI (GitHub Actions, Ubuntu 24.04) runs the tests, then publishes to GitHub Releases the `.deb`, the static binary and `SHA256SUMS`, and pushes `ghcr.io/midnights-ra1n/screensharex:X.Y.Z` (also tagged `X.Y`). Every push to a branch also publishes an image tagged with the branch name.

---

<div align="center">
<sub>Inspired by <a href="https://github.com/SergiX44/XBackBone">XBackBone</a> · Made for people who screenshot too much.</sub>
</div>
