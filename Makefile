.PHONY: init-env install build clean start update shutdown prune teardown urls test format lint check export-image

PROJECT := aws-ddns
APPS    := aws-ddns
# Version stamped into the binary and used as the image tag. The VERSION file
# is the source of truth (see AGENTS.md → Versioning); override only for
# experiments: make export-image VERSION=x.y.z-test
VERSION ?= $(shell cat VERSION 2>/dev/null || echo dev)
DIST    := dist
# Container engine — detect Podman first, fall back to Docker. Every Compose/build/run recipe
# MUST go through $(DOCKER), never a hardcoded `docker`/`podman`, so the repo runs on whichever
# engine is installed.
DOCKER  := $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
COMPOSE := $(DOCKER) compose -f infra/local/docker-compose.yaml --env-file .env

# ── Environment ───────────────────────────────────────────────────────────────

init-env:
	@if [ ! -f .env ]; then \
	  cp .env.example .env && echo "Created .env from .env.example"; \
	else \
	  echo "Syncing .env with .env.example…"; \
	  if [ -s .env ] && [ "$$(tail -c1 .env | wc -l)" -eq 0 ]; then echo "" >> .env; fi; \
	  while IFS= read -r line; do \
	    key=$$(printf '%s' "$$line" | sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p'); \
	    [ -z "$$key" ] && continue; \
	    if ! grep -q "^$$key=" .env; then echo "$$line" >> .env && echo "  + $$key"; fi; \
	  done < .env.example; \
	  while IFS= read -r line; do \
	    key=$$(printf '%s' "$$line" | sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p'); \
	    [ -z "$$key" ] && continue; \
	    grep -q "^$$key=" .env.example || { tmp=$$(mktemp); grep -v "^$$key=" .env > "$$tmp" && mv "$$tmp" .env && echo "  - $$key"; }; \
	  done < .env; \
	  echo "Done"; \
	fi

# ── Workspace ─────────────────────────────────────────────────────────────────

install:
	@for app in $(APPS); do echo "▶ $$app install" && $(MAKE) -C apps/$$app install || exit 1; done

build:
	@for app in $(APPS); do echo "▶ $$app build" && $(MAKE) -C apps/$$app build || exit 1; done

clean:
	@for app in $(APPS); do echo "▶ $$app clean" && $(MAKE) -C apps/$$app clean || exit 1; done

test:
	@for app in $(APPS); do echo "▶ $$app test" && $(MAKE) -C apps/$$app test || exit 1; done

format:
	@for app in $(APPS); do echo "▶ $$app format" && $(MAKE) -C apps/$$app format || exit 1; done

lint:
	@for app in $(APPS); do echo "▶ $$app lint" && $(MAKE) -C apps/$$app lint || exit 1; done

check: clean install format lint build test

# ── Docker ────────────────────────────────────────────────────────────────────
# `start` and `update` perform the SAME operations (a single shared recipe). --ignore-buildable
# only pulls registry images; the locally-built aws-ddns image is (re)built by `up --build`.

start update:
	$(COMPOSE) pull --ignore-buildable
	$(COMPOSE) up -d --build
	@$(MAKE) --no-print-directory urls

shutdown:
	$(COMPOSE) down

prune:
	$(COMPOSE) down --rmi local --remove-orphans

teardown:
	$(COMPOSE) down -v --rmi all --remove-orphans

# ── Manual distribution (no registry) ─────────────────────────────────────────
# Build the deployment image for EVERY supported architecture, pack each into a
# compressed archive with a checksum, and drop them in dist/. On the target
# machine `uname -m` picks the archive: x86_64 → amd64, aarch64 → arm64.
# Import with `docker load` (or `podman load`). No ECR/registry.
# A wrong-arch image imports fine but dies instantly with NO logs (`exec format
# error`) — that is exactly what the per-arch archives + the -version probe below
# are here to rule out.

PLATFORM_ARCHS := amd64 arm64
# Fully-qualified image name. Podman normalizes unqualified tags to localhost/…
# and `save` embeds the name, so `docker load` on the target restores EXACTLY
# this — the compose file and every documented command must use the same name,
# or Docker resolves it against Docker Hub and never finds the loaded image.
IMAGE := localhost/aws-ddns

export-image:
	@mkdir -p $(DIST)
	@set -e; for arch in $(PLATFORM_ARCHS); do \
	  echo "▶ building $(IMAGE):$(VERSION) for linux/$$arch"; \
	  $(DOCKER) build --platform linux/$$arch --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) apps/aws-ddns; \
	  artifact=aws-ddns-$(VERSION)-linux-$$arch.tar.gz; \
	  echo "▶ exporting $(DIST)/$$artifact"; \
	  $(DOCKER) save $(IMAGE):$(VERSION) | gzip > $(DIST)/$$artifact; \
	  ( cd $(DIST) && { command -v sha256sum >/dev/null 2>&1 && sha256sum "$$artifact" > "$$artifact.sha256" || shasum -a 256 "$$artifact" > "$$artifact.sha256"; } ); \
	done; \
	sed 's/__VERSION__/$(VERSION)/' infra/server/docker-compose.yaml > $(DIST)/docker-compose.yaml; \
	printf '\n%s\n' "Exported to $(DIST)/:"; \
	ls -lh $(DIST)/aws-ddns-$(VERSION)-linux-*.tar.gz $(DIST)/aws-ddns-$(VERSION)-linux-*.sha256 $(DIST)/docker-compose.yaml | awk '{print "  " $$0}'; \
	printf '\n%s\n  %s\n  %s\n  %s\n  %s\n%s\n  %s\n  %s\n\n' \
	  "On the target, run 'uname -m' to pick the archive (x86_64 → amd64, aarch64 → arm64), copy it with its .sha256, then:" \
	  "sha256sum -c <archive>.sha256          # verify the copy" \
	  "docker load -i <archive>               # restores the image as $(IMAGE):$(VERSION)" \
	  "docker image inspect $(IMAGE):$(VERSION) --format '{{.Architecture}}'   # must match the machine" \
	  "docker run --rm $(IMAGE):$(VERSION) -version   # must print $(VERSION) — proves it executes" \
	  "Then deploy $(DIST)/docker-compose.yaml as a Compose project (or create the container in your server's container UI) after editing its host data-folder path," \
	  "or run it directly:" \
	  "docker run -d --name aws-ddns --restart unless-stopped --read-only --cap-drop ALL -v /srv/aws-ddns:/var/lib/aws-ddns $(IMAGE):$(VERSION)"
	@printf '%s\n  %s\n\n' \
	  "Upgrading an existing deployment: load the new archive, then RECREATE the container with the new tag (delete + create, or compose re-deploy)." \
	  "A platform's 'update'/pull action cannot work without a registry — it fails harmlessly; recreating is the upgrade path. Data survives in the host data folder."

# aws-ddns is a headless daemon: no inbound port, nothing browser-reachable.
urls:
	@printf '\n%s\n%s\n\n' \
	  "aws-ddns is a headless daemon — it exposes no ports and no URLs." \
	  "Follow its logs with: $(DOCKER) compose -f infra/local/docker-compose.yaml --env-file .env logs -f aws-ddns"

# This repository intentionally has no e2e app and no database, so the canonical
# e2e-run / e2e-clean / db-migrate / db-reset targets are absent — see AGENTS.md
# → "Intentional deviations".
