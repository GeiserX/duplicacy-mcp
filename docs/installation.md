# Installation

## Quick-start (Docker Compose)

```yaml
services:
  duplicacy-mcp:
    image: drumsergio/duplicacy-mcp:0.1.0
    ports:
      - "127.0.0.1:8080:8080"
    environment:
      - DUPLICACY_EXPORTER_URL=http://duplicacy-exporter:9750
```

> **Security note:** The HTTP transport listens on `127.0.0.1:8080` by default. If you need to expose it on a network, place it behind a reverse proxy with authentication.

## Install via npm (stdio transport)

```sh
npx duplicacy-mcp
```

Or install globally:

```sh
npm install -g duplicacy-mcp
duplicacy-mcp
```

This downloads the pre-built Go binary from GitHub Releases for your platform and runs it with stdio transport. Requires at least one [published release](https://github.com/GeiserX/duplicacy-mcp/releases).

## Local build

```sh
git clone https://github.com/GeiserX/duplicacy-mcp
cd duplicacy-mcp

# (optional) create .env from the sample
cp .env.example .env && $EDITOR .env

go run ./cmd/server
```

