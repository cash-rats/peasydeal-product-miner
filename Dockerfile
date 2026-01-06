# Template image for running the Go runner.
# NOTE: You still need to install/configure `codex` and ensure MCP is configured at runtime.

FROM golang:1.22-bookworm AS build

WORKDIR /src
COPY runner/ /src/runner/
RUN cd /src/runner && go build -o /out/runner .

FROM debian:bookworm-slim

WORKDIR /app

COPY config/ /app/config/

COPY --from=build /out/runner /app/runner

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates nodejs npm \
  && rm -rf /var/lib/apt/lists/*

RUN chmod +x /app/runner

# Expected runtime mounts:
# - /out for results
# - /codex for ~/.codex (HOME=/codex)
CMD ["/app/runner", "--help"]
