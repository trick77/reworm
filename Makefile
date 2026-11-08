.PHONY: build e2e run test tidy ui-build ui-dev ui-test dataset import-cards import-songs

# go:embed cannot reach ../data: the server embeds a copy of the dataset (ignored), as the image build makes it.
dataset:
	cp data/songs.jsonl backend/internal/dataset/files/songs.jsonl

# The song database: card snapshots from the lists named in .env.local, then the songs. The song
# step takes about an hour on an empty data/cache (MusicBrainz, 1 req/s); a rerun answers from the cache.
import-cards:
	cd backend && go run ./cmd/import -env ../.env.local -data ../data cards

import-songs:
	cd backend && go run ./cmd/import -env ../.env.local -data ../data songs

tidy:
	cd backend && go mod tidy

test:
	cd backend && go test ./...

# npm ci only when the lockfile is newer than the last install (npm rewrites node_modules/.package-lock.json).
ui/node_modules/.package-lock.json: ui/package-lock.json
	cd ui && npm ci

ui-test: ui/node_modules/.package-lock.json
	cd ui && npm run test

# vite builds into backend/web/dist and does not empty it (emptyOutDir: false): the old hashed bundles would be
# embedded too.
ui-build: ui/node_modules/.package-lock.json
	find backend/web/dist -mindepth 1 ! -name .gitkeep -exec rm -rf {} +
	cd ui && npm run build

# No version stamp: unstamped, kit's version.String reports dev-<commit>[-dirty]; releases stamp it in the image.
build: ui-build dataset
	cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../bin/reworm ./cmd/reworm

# The server on :8080 with its database in data/dev (ignored); `make ui-dev` beside it for vite's dev server.
run: dataset
	cd backend && BACKEND_DATA_DIR=../data/dev go run ./cmd/reworm

ui-dev: ui/node_modules/.package-lock.json
	cd ui && npm run dev

# Playwright (chromium + webkit) against the real binary on a free port, fresh data dir, dev clips and fixture
# songs (BACKEND_DEV_*). Not part of check. Browsers once: cd ui && npx playwright install chromium webkit
e2e: ui/node_modules/.package-lock.json
	cd ui && npm run e2e

# ship:begin (owned by ship init)
# The gate: everything CI checks, configured in ship.yaml. Master's ship, as the pre-push hook and the @master
# workflows run it: its sha asked of GitHub (a proxy lags on @master). A local ship: SHIP_CMD=<go-installed binary>,
# one name for here and the hook, so make check and the push gate never run different ships.
# Exactly refs/heads/master (ls-remote matches by suffix), a 40-hex sha or nothing runs, as in the workflows.
SHIP_CMD ?= sha=$$(git ls-remote https://github.com/trick77/ship refs/heads/master | \
		awk '$$2 == "refs/heads/master" { print $$1 }'); \
	echo "$$sha" | grep -Eqx '[0-9a-f]{40}' || \
		{ echo "cannot resolve ship's master on GitHub: '$$sha' (git ls-remote, error above)"; exit 1; }; \
	go run github.com/trick77/ship@$$sha

.PHONY: check hooks lint

check:
	$(SHIP_CMD) check

hooks:
	$(SHIP_CMD) hooks

# Quick lint while editing, at the version check runs.
lint:
	$(SHIP_CMD) lint
# ship:end
