build:
	go build -ldflags="-s -w" -o kc .

build-all:
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o dist/kc-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w" -o dist/kc-linux-arm64 .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w" -o dist/kc-darwin-amd64 .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o dist/kc-darwin-arm64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/kc-windows-amd64.exe .
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o dist/kc-windows-arm64.exe .

clean:
	rm -f kc dist/*

# Install a self-rebuilding `kc` launcher to /usr/local/bin so the command on your PATH is
# never stale: it rebuilds dist/kc-linux-amd64 whenever a *.go file (or go.mod) is newer than
# the binary, then execs it. The repo path is baked in via $(CURDIR). Needs sudo.
dev-install:
	@tmp=$$(mktemp); \
	{ \
	  printf '%s\n' '#!/usr/bin/env bash'; \
	  printf '%s\n' 'set -uo pipefail'; \
	  printf '%s\n' '# Auto-rebuilding kc launcher (installed by `make dev-install`). Edit there, not here.'; \
	  printf '%s\n' 'CLI_DIR="$(CURDIR)"'; \
	  printf '%s\n' 'BIN="$$CLI_DIR/dist/kc-linux-amd64"'; \
	  printf '%s\n' 'if [ ! -x "$$BIN" ] || [ -n "$$(find "$$CLI_DIR" -maxdepth 1 -name "*.go" -newer "$$BIN" -print -quit 2>/dev/null)" ] || [ "$$CLI_DIR/go.mod" -nt "$$BIN" ]; then'; \
	  printf '%s\n' '  ( cd "$$CLI_DIR" && go build -ldflags="-s -w" -o "$$BIN" . ) 1>&2 || echo "kc: rebuild failed; using existing binary" >&2'; \
	  printf '%s\n' 'fi'; \
	  printf '%s\n' 'exec "$$BIN" "$$@"'; \
	} > "$$tmp"; \
	sudo install -m 0755 "$$tmp" /usr/local/bin/kc; \
	rm -f "$$tmp"; \
	echo "Installed auto-rebuild kc -> $(CURDIR)/dist/kc-linux-amd64"

.PHONY: build build-all clean dev-install
