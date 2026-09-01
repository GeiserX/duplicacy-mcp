# Stable Go, not a release candidate. This builder compiles the binaries that
# ship to Docker Hub, because .goreleaser.yaml passes the source in through
# extra_files and builds both published images from this file. 1.27rc3 was
# pushed 2026-08-16 and 1.27.0 on 2026-08-30, so the RC pin was two weeks
# behind final. CI builds linux/arm64 on pull requests, so a bad bump here
# goes red before merge rather than during a release.
FROM golang:1.27 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/duplicacy-mcp ./cmd/server

FROM alpine:3.24
LABEL io.modelcontextprotocol.server.name="io.github.GeiserX/duplicacy-mcp"
COPY --from=builder /out/duplicacy-mcp /usr/local/bin/duplicacy-mcp
EXPOSE 8080
ENV LISTEN_ADDR=0.0.0.0:8080
ENV DUPLICACY_EXPORTER_URL=http://duplicacy-exporter:9750
ENTRYPOINT ["/usr/local/bin/duplicacy-mcp"]
