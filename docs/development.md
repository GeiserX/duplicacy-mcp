# Development

<p>
  <a href="https://codecov.io/gh/GeiserX/duplicacy-mcp"><img src="https://codecov.io/gh/GeiserX/duplicacy-mcp/graph/badge.svg" alt="codecov"/></a>
  <img src="https://img.shields.io/badge/Go-1.27-blue?style=flat-square&logo=go&logoColor=white" alt="Go"/>
</p>

## Testing

`go test -race ./...` covers the exporter client, every tool, the resources, the HTTP protections
(token, Origin, body limit) and a protocol smoke test that drives the real stdio transport with both a
2026-07-28 `server/discover` and a legacy `initialize`. `node --test` covers the npm wrapper's download
checks. Before making a PR, also try the server in [Inspector](https://modelcontextprotocol.io/docs/tools/inspector).

## Credits

[Duplicacy](https://duplicacy.com/) -- lock-free deduplication cloud backup

[duplicacy-exporter](https://github.com/GeiserX/duplicacy-exporter) -- Prometheus exporter for Duplicacy

[MCP-GO](https://github.com/mark3labs/mcp-go) -- modern MCP implementation

[GoReleaser](https://goreleaser.com/) -- painless multi-arch releases

## Maintainers

[@GeiserX](https://github.com/GeiserX).

## Contributing

Feel free to dive in! [Open an issue](https://github.com/GeiserX/duplicacy-mcp/issues/new) or submit PRs.

Duplicacy-MCP follows the [Contributor Covenant](http://contributor-covenant.org/version/2/1/) Code of Conduct.

