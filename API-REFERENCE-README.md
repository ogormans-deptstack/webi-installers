# Comprehensive API & Architecture Reference Available

Another agent has written a complete reference document covering the entire
webinstall.dev + webi-installers public API, architecture, and internals.

## Where to find it

The full documentation is at:

```
/Users/aj/.claude/projects/-Users-aj-Projects-claude-webinstall-dev/memory/api-reference.md
```

## What it covers

1. **Three domains** — webinstall.dev, webi.sh, webi.ms: routing, hostname logic
2. **All API endpoints** — /api/releases, /api/installers, /api/stats, /api/debug
   - Every query param, path param, header, response format
   - JSON and TSV response schemas with examples
3. **User-Agent detection** — exact regex patterns, OS/arch/libc mapping tables
4. **CLI vs browser detection** — how the same URL serves scripts or HTML
5. **Installer script generation** — complete flow from HTTP request to rendered script
6. **Bootstrap templates** — curl-pipe-bootstrap.tpl.sh/.ps1 structure and variables
7. **Package structure** — releases.js, install.sh, install.ps1 patterns
8. **Release normalization** — how OS/arch/ext are auto-detected from filenames
9. **Platform fallback chains** — arm64→amd64, armv7l→armv6l, triplet matching
10. **28 template variables** — complete reference with examples
11. **Security & validation** — input sanitization, shell injection prevention
12. **Statistics tracking** — file-based stats paths and formats
13. **Configuration** — env vars, URL derivation, dotenv files
14. **Dual-purpose HTML/script trick** — how webi.sh serves both browsers and shells
15. **Install path conventions** — ~/.local/opt, ~/.local/bin, envman integration
16. **Complete request flow examples** — browser, curl|sh, and API call traces

## Source codebases analyzed

- `/Users/aj/Projects/claude/webinstall.dev/` (main branch) — the Express server
- `/Users/aj/Projects/claude/webi-installers/` (main branch) — installer packages + core _webi/ infrastructure

Read the full doc before making architectural decisions for the Go rewrite.
