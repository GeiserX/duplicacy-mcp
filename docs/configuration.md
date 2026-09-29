# Configuration

| Variable                 | Default                    | Description                                          |
|--------------------------|----------------------------|------------------------------------------------------|
| `DUPLICACY_EXPORTER_URL` | `http://localhost:9750`    | Duplicacy Prometheus exporter URL (without trailing /)|
| `LISTEN_ADDR`            | `127.0.0.1:8080`           | HTTP listen address (Docker sets `0.0.0.0:8080`)     |
| `TRANSPORT`              | _(empty = HTTP)_           | Set to `stdio` for stdio transport                   |

Put them in a `.env` file (from `.env.example`) or set them in the environment.

## Client configuration

Over stdio, which the npm package always uses, a client that reads an `mcpServers` block starts the
server itself:

```json
{
  "mcpServers": {
    "duplicacy": {
      "command": "npx",
      "args": ["-y", "duplicacy-mcp"],
      "env": { "DUPLICACY_EXPORTER_URL": "http://localhost:9750" }
    }
  }
}
```

Over HTTP, for a server you already run (the Docker image, or the binary without `TRANSPORT=stdio`), a
client that supports remote servers connects to the `/mcp` endpoint:

```json
{
  "mcpServers": {
    "duplicacy": { "url": "http://127.0.0.1:8080/mcp" }
  }
}
```

The HTTP transport has no authentication of its own; keep it on `127.0.0.1` or put a reverse proxy with
authentication in front.
