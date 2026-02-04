# Template image for running the Go devtool.
# NOTE: You still need to install/configure `codex`/`gemini` and ensure MCP is configured at runtime.

FROM golang:1.25-bookworm AS build

WORKDIR /src
COPY go.mod go.sum /src/
RUN go mod download

COPY cmd/ /src/cmd/
COPY internal/ /src/internal/
COPY config/ /src/config/
COPY db/ /src/db/
COPY cache/ /src/cache/

RUN go build -o /out/worker ./cmd/worker
RUN go build -o /out/devtool ./cmd/devtool

FROM node:20-bookworm-slim

WORKDIR /app

COPY config/ /app/config/
COPY entrypoint.sh /app/entrypoint.sh

COPY --from=build /out/worker /app/worker
COPY --from=build /out/devtool /app/devtool

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl \
  && rm -rf /var/lib/apt/lists/*

# Install Codex and Gemini CLI inside the image (requires network access during build).
# If your package names differ, override at build time:
#   docker build --build-arg CODEX_NPM_PKG=... --build-arg GEMINI_NPM_PKG=... .
ARG CODEX_NPM_PKG=@openai/codex
ARG GEMINI_NPM_PKG=@google/gemini-cli
RUN npm install -g "${CODEX_NPM_PKG}" "${GEMINI_NPM_PKG}"

RUN chmod +x /app/worker
RUN chmod +x /app/devtool
RUN chmod +x /app/entrypoint.sh

# Expected runtime mounts:
# - /out for results
# - /codex for Codex credential store (bind-mounted)
# - /gemini for Gemini credential store + settings (bind-mounted)
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["/app/devtool", "--help"]
