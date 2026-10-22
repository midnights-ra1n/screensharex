VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
REPO ?= midnights-ra1n/screensharex
LDFLAGS = -s -w -X main.version=$(VERSION) -X main.repo=$(REPO)

.PHONY: web build test deb dev
web:
	cd web && npm ci && npm run build

build: web
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/screensharex-linux-amd64 .

test: web
	go vet ./... && go test ./...

deb: build
	packaging/build-deb.sh dist/screensharex-linux-amd64 $(VERSION) $(REPO) dist

# Single UI on :8080, served from web/dist and rebuilt on every frontend change (refresh the page).
dev:
	cd web && npm run watch & SSX_DATA=./data SSX_WEB_DIR=web/dist go run .
