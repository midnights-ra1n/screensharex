package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

const uploadScript = `#!/usr/bin/env bash
# ScreenShareX uploader — usage: screensharex-upload FILE [FILE...]
# Works with Spectacle (Exporter > Ouvrir avec), ScreenCloud (plugin Shell Script), cron, scripts…
set -uo pipefail
SSX_URL="${SSX_URL:-__BASE__}"
SSX_TOKEN="${SSX_TOKEN:-__TOKEN__}"
[ $# -gt 0 ] || { echo "usage: $(basename "$0") FICHIER..." >&2; exit 1; }

notify() {
  if command -v notify-send >/dev/null; then notify-send -a ScreenShareX "ScreenShareX" "$1"
  elif command -v osascript >/dev/null; then osascript -e "display notification \"$1\" with title \"ScreenShareX\""
  fi
}
copy() {
  for c in wl-copy "xclip -selection clipboard" "xsel -b" pbcopy; do
    command -v ${c%% *} >/dev/null && { printf '%s' "$1" | $c; return; }
  done
}

status=0
for f in "$@"; do
  resp=$(curl -sS -H "Authorization: Bearer $SSX_TOKEN" -F "file=@\"$f\"" "$SSX_URL/api/upload")
  url=$(printf '%s' "$resp" | sed -n 's/.*"url":"\([^"]*\)".*/\1/p')
  if [ -n "$url" ]; then
    echo "$url"; copy "$url"; notify "Lien copié : $url"
  else
    err=$(printf '%s' "$resp" | sed -n 's/.*"error":"\([^"]*\)".*/\1/p')
    echo "Échec pour $f : ${err:-$resp}" >&2; notify "Échec : ${err:-envoi impossible}"; status=1
  fi
done
exit $status
`

const desktopEntry = `[Desktop Entry]
Type=Application
Name=ScreenShareX
Comment=Envoyer vers ScreenShareX et copier le lien
Exec=screensharex-upload %F
Icon=document-send
Terminal=false
MimeType=image/png;image/jpeg;image/gif;image/webp;video/mp4;video/webm;video/x-matroska;
Categories=Utility;
`

func (a *App) clientConfig(w http.ResponseWriter, r *http.Request, u *User) {
	base := a.base(r)
	host := base
	if p, err := url.Parse(base); err == nil {
		host = p.Host
	}
	var body []byte
	name := r.PathValue("name")
	switch name {
	case "sharex.sxcu", "xerahs.sxcu": // XerahS reads ShareX custom uploader files
		body, _ = json.MarshalIndent(map[string]any{
			"Version": "17.0.0", "Name": "ScreenShareX (" + host + ")",
			"DestinationType": "ImageUploader, FileUploader", "RequestMethod": "POST",
			"RequestURL": base + "/api/upload", "Headers": map[string]string{"Authorization": "Bearer " + u.APIToken},
			"Body": "MultipartFormData", "FileFormName": "file",
			"URL": "{json:url}", "ThumbnailURL": "{json:thumb}", "DeletionURL": "{json:delete_url}", "ErrorMessage": "{json:error}",
		}, "", "  ")
	case "ishare.iscu":
		body, _ = json.MarshalIndent(map[string]any{
			"name": "ScreenShareX (" + host + ")", "requestURL": base + "/api/upload",
			"headers":      map[string]string{"Authorization": "Bearer " + u.APIToken},
			"fileFormName": "file", "requestBodyType": "multipartFormData",
			"responseURL": "{{url}}", "deletionURL": "{{delete_url}}", "deleteRequestType": "DELETE",
		}, "", "  ")
	case "screensharex-upload":
		body = []byte(strings.NewReplacer("__BASE__", base, "__TOKEN__", u.APIToken).Replace(uploadScript))
	case "screensharex.desktop":
		body = []byte(desktopEntry)
	default:
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Write(body)
}
