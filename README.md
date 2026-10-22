# ScreenShareX

Hébergeur auto-hébergé de captures d'écran, GIF et vidéos, à la XBackBone : un seul binaire Go (SQLite + interface TypeScript embarquée), compatible ShareX, XerahS, ishare, ScreenCloud, Spectacle, la feuille de partage macOS et la ligne de commande.

## Installation

### Debian / Ubuntu (.deb)

```sh
sudo apt install ./screensharex_X.Y.Z_amd64.deb
```

Le service écoute sur `:8080` et stocke tout dans `/var/lib/screensharex`. La configuration se trouve dans `/etc/default/screensharex`. Les mises à jour s'installent depuis **Admin → Mises à jour**.

### Docker

```sh
docker compose up -d   # ScreenShareX + cache Valkey, voir docker-compose.yml
```

Pour mettre à jour : `docker compose pull && docker compose up -d`.

Ouvrez ensuite l'interface : **le premier compte créé devient administrateur**. Créez-le dès l'installation.

## Configuration

| Variable | Défaut | Rôle |
|---|---|---|
| `SSX_ADDR` | `:8080` | Adresse d'écoute |
| `SSX_DATA` | `./data` | Base SQLite et fichiers |
| `SSX_BASE_URL` | déduite de la requête | URL publique des liens (à définir derrière un reverse proxy) |
| `SSX_REDIS` | cache mémoire | `redis://[:mdp@]hôte:6379/0`, pour Valkey, KeyDB, Dragonfly ou Redis |

Les réglages suivants se modifient depuis l'interface d'administration : ouverture des inscriptions, quota par défaut, quota par utilisateur, taille maximale par fichier, nombre d'envois par heure et compression.

## Fonctionnement

- **Déduplication** : chaque fichier est identifié par son SHA-256 et stocké une seule fois sur le disque. Si un utilisateur renvoie un fichier identique, il récupère son lien existant.
- **Versions légères** : si ffmpeg est installé, les images sont converties en WebP 1600 px et les GIF et vidéos en MP4 H.264 480p. Elles sont servies en mode « Éco data » ou aux navigateurs qui envoient `Save-Data`.
- **Embeds** : titre, description, couleur et nom du site sont personnalisables. Un mode « brut » envoie directement le fichier à Discord et aux autres bots. Les vidéos leur sont toujours envoyées brutes, pour être lues directement dans l'application.
- **Sauvegarde** : l'export produit un `.tar.gz` avec la base et les fichiers. L'import remplace la base, ajoute les fichiers puis redémarre le serveur.

## API

```sh
curl -H "Authorization: Bearer <jeton>" -F "file=@capture.png" https://img.example.com/api/upload
# → {"url": "...", "raw": "...", "thumb": "...", "delete_url": "...", "media": {...}}
```

## Développement

```sh
make dev                     # backend sur :8080
cd web && npm run dev        # interface sur :5173 (proxy /api)
make test                    # build du frontend + go vet + go test
make deb VERSION=v0.1.0      # dist/screensharex_0.1.0_amd64.deb
```

Pour publier une release, poussez un tag `vX.Y.Z`. La CI (Ubuntu 24.04) publie alors le `.deb`, le binaire, `SHA256SUMS` et l'image `ghcr.io/midnights-ra1n/screensharex`.
