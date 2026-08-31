FROM golang:1.27rc3 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# TEMPORARY positive control - removed in the next commit.
# A genuine linux/arm64 build reports GOARCH=arm64, so this must fail. If the
# job goes green here, it is not really building arm64 and the gate is inert.
RUN echo "GOARCH=$(go env GOARCH)" && test "$(go env GOARCH)" = "amd64"
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/duplicacy-mcp ./cmd/server

FROM alpine:3.24
LABEL io.modelcontextprotocol.server.name="io.github.GeiserX/duplicacy-mcp"
COPY --from=builder /out/duplicacy-mcp /usr/local/bin/duplicacy-mcp
EXPOSE 8080
ENV LISTEN_ADDR=0.0.0.0:8080
ENV DUPLICACY_EXPORTER_URL=http://duplicacy-exporter:9750
ENTRYPOINT ["/usr/local/bin/duplicacy-mcp"]
