# Both base images are pinned by digest; dependabot's docker ecosystem keeps
# the digests current. This builder compiles the binaries that ship to Docker
# Hub, because .goreleaser.yaml passes the source in through extra_files and
# builds both published images from this file. CI builds linux/arm64 on pull
# requests, so a bad bump here goes red before merge rather than during a
# release. go1.27.3 rejects the compromised eMudhra (emSign) intermediates.
FROM golang:1.27.3@sha256:2920bb124f1a2e12a1d39f0cab756ae0068441ee1da7655dec7f5e374944095b AS builder
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/geiserx/duplicacy-mcp/version.Version=${VERSION} -X github.com/geiserx/duplicacy-mcp/version.Commit=${COMMIT} -X github.com/geiserx/duplicacy-mcp/version.Date=${DATE}" \
      -o /out/duplicacy-mcp ./cmd/server

FROM alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
# The base image can lag the CA bundle; take the current one, which drops
# the compromised eMudhra (emSign) intermediates.
RUN apk upgrade --no-cache ca-certificates-bundle
LABEL io.modelcontextprotocol.server.name="io.github.GeiserX/duplicacy-mcp"
COPY --from=builder /out/duplicacy-mcp /usr/local/bin/duplicacy-mcp
USER 65532:65532
EXPOSE 8080
# The container listens on every interface, so the server refuses to start
# until MCP_AUTH_TOKEN is set (or MCP_ALLOW_UNAUTHENTICATED=true behind an
# authenticating reverse proxy).
ENV LISTEN_ADDR=0.0.0.0:8080
ENV DUPLICACY_EXPORTER_URL=http://duplicacy-exporter:9750
ENTRYPOINT ["/usr/local/bin/duplicacy-mcp"]
