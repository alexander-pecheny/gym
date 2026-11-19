set dotenv-load := true

default: dev

# gym has no TypeScript of its own; the kit's login and menu scripts are the
# only browser code it serves. dopesuite does not commit them, so they are
# built inside the submodule before anything that compiles the kit.
build-web:
    go -C dopesuite/scripts/webbuild run . uikit

build: build-web
    CGO_ENABLED=0 go build -o gym-server ./cmd/gym-server

# A local instance reads the coach's sheet if GYM_SHEET_CSV is set (.env).
dev: build-web
    GYM_ENV=development go run ./cmd/gym-server

generate: build-web
    go generate ./...

fmt:
    #!/usr/bin/env bash
    set -euo pipefail
    mapfile -t files < <(find . -path ./dopesuite -prune -o -type f -name '*.go' -print)
    ((${#files[@]} == 0)) || gofmt -w "${files[@]}"

vet: build-web
    go vet ./...

tidy-check:
    go mod tidy -diff

# The generated files must match what the generators make now.
generate-check: generate
    git diff --exit-code -- i18nstrings web/ui

test: build-web
    go test ./...

check:
    just fmt
    just vet
    just tidy-check
    just generate-check
    just test

# Build for vps-he and swap the binary in (deploy/deploy.sh).
deploy: build-web
    deploy/deploy.sh
