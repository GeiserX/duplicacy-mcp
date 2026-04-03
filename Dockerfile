FROM golang:1.26 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/duplicacy-mcp ./cmd/server

FROM alpine:3.23
LABEL io.modelcontextprotocol.server.name="io.github.GeiserX/duplicacy-mcp"
COPY --from=builder /out/duplicacy-mcp /usr/local/bin/duplicacy-mcp
EXPOSE 8080
ENV LISTEN_ADDR=0.0.0.0:8080
ENV DUPLICACY_EXPORTER_URL=http://duplicacy-exporter:9750
ENTRYPOINT ["/usr/local/bin/duplicacy-mcp"]
