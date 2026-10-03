# Dev/test image for the pictionary app (vendored from scribble-rs/scribble.rs).
#
# This is container B: the project's dev/test environment. Compose is the run
# target - there is intentionally no separate runtime image here. Upstream's
# scratch-based release image still lives in linux.Dockerfile, untouched.
#
# Go version is pinned to match .github/workflows (1.25.5) and go.mod
# (go 1.25.0). The frontend is vanilla JS embedded via go:embed, so Go is the
# only toolchain this project needs - no Node, no bundler.

FROM golang:1.25.5-bookworm

# GOTOOLCHAIN=local  - go.mod asks for 1.25.0, so never download another
#                      toolchain; the pinned image already satisfies it.
# GOFLAGS            - the source is mounted from the host, where VCS stamping
#                      can fail on dubious ownership. Build metadata is
#                      injected with ldflags by linux.Dockerfile instead.
#
# CGO is deliberately left enabled (the image default): the race detector that
# `go test -race` depends on requires cgo. The static CGO_ENABLED=0 build
# belongs to linux.Dockerfile, not to this dev image.
ENV GOTOOLCHAIN=local \
    GOFLAGS=-buildvcs=false

WORKDIR /app

# --- dependencies -----------------------------------------------------------
# go.mod / go.sum change rarely, so this layer is reused across every rebuild
# that only touches application code.
COPY go.mod go.sum ./
RUN go mod download

# --- application code -------------------------------------------------------
COPY . .

# Upstream default (internal/config/config.go: Port: 8080).
ENV PORT=8080
EXPOSE 8080

# No hot-reload watcher: the frontend, word lists and translations are all
# go:embed'd, so any code change needs a restart regardless. Run
# `docker compose restart app` after editing Go or embedded assets.
CMD ["go", "run", "./cmd/scribblers"]
