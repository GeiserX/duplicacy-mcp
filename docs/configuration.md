# Configuration

| Variable                 | Default                    | Description                                          |
|--------------------------|----------------------------|------------------------------------------------------|
| `DUPLICACY_EXPORTER_URL` | `http://localhost:9750`    | Duplicacy Prometheus exporter URL (without trailing /)|
| `LISTEN_ADDR`            | `127.0.0.1:8080`           | HTTP listen address (Docker sets `0.0.0.0:8080`)     |
| `TRANSPORT`              | _(empty = HTTP)_           | Set to `stdio` for stdio transport                   |

Put them in a `.env` file (from `.env.example`) or set them in the environment.

