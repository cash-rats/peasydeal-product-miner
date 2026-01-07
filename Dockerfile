# Template image for running the Go runner.
# NOTE: You still need to install/configure `codex` and ensure MCP is configured at runtime.

FROM golang:1.22-bookworm AS build

WORKDIR /src
COPY go.mod /src/go.mod
COPY internal/ /src/internal/
COPY cmd/runner/ /src/cmd/runner/
RUN cd /src && go build -o /out/runner ./cmd/runner

FROM debian:bookworm-slim

WORKDIR /app

COPY config/ /app/config/

COPY --from=build /out/runner /app/runner

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates nodejs npm \
  && rm -rf /var/lib/apt/lists/*

# Install Codex CLI inside the image (requires network access during build).
# If your Codex package name differs, override at build time:
#   docker build --build-arg CODEX_NPM_PKG=... .
ARG CODEX_NPM_PKG=@openai/codex
RUN npm install -g "${CODEX_NPM_PKG}"

RUN chmod +x /app/runner

# Expected runtime mounts:
# - /out for results
# - /codex for ~/.codex (HOME=/codex)
CMD ["/app/runner", "--help"]
