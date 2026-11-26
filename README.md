# th0ughts

A single-owner microblog with two pages. Readers open the timeline at `/`. The owner opens `/login` directly, signs in, and writes, translates, uploads media, changes themes, and manages the site on the same timeline.

Go serves the initial HTML and the embedded React application. Reading, searching, pagination, per-post links, RSS, and Markdown work without JavaScript. A deployed instance runs one application process.

## Start with SQLite

From the repository root, create a persistent master key and start the service.

```sh
umask 077
openssl rand -hex 32 > master.key
export TH0UGHTS_MASTER_KEY="$(cat master.key)"
docker compose -p thoughts -f deploy/compose.sqlite.yaml up -d --build
```

Open `http://localhost:8080/login`. Enter the master key, choose an owner password on the timeline, and open **Settings → Site** to set your public URL, author, and default language. Use an HTTPS domain or `localhost` for passkeys. For an internet deployment, terminate TLS at a reverse proxy. Keep the master key and the data volume across restarts. Keep the key outside your checkout when distributing or archiving the repository.

The application requires one environment variable, `TH0UGHTS_MASTER_KEY`. The initial database locator is encrypted in the data directory. Site preferences, translations, themes, accounts, and credentials live in the selected store. Uploaded files use disk or an S3-compatible service.

## Publishing

Choose **New post**, enter Markdown and optional tags or attachments, and select **Publish** or **Save draft**. English and Turkish tabs edit translations of the same post. **Original language** controls the fallback when a translation has not been published. The original must be published for the post to appear publicly. **Edit** opens its current revision. Concurrent edits return a conflict so another client's changes survive.

The language in a shared URL takes precedence over a saved reader preference, followed by the owner's default. A post keeps one ID across languages, theme changes, and backup restores. Public HTML contains canonical and alternate links, Open Graph, social cards, and JSON-LD. The default social image is generated locally.

| Resource | Address |
| --- | --- |
| Timeline and focused post | `/?lang=en`, `/?post=<id>&lang=tr` |
| Markdown | `/feed.md`, `/post/<id>.md`, or `Accept: text/markdown` on the timeline |
| RSS and discovery | `/feed.xml?lang=tr`, `/sitemap.xml`, `/robots.txt` |
| REST and OpenAPI | `/api/v1/posts`, `/api/v1/openapi.json` |
| Agent discovery | `/api/v1/tools?lang=en`, `/mcp`, browser WebMCP |
| Health | `/healthz` |

## Customize and deploy

- Upload a single HTML file from **Settings → Appearance**. Preview it, select its options, and save. Community themes use the same file format. [Theme authoring guide](docs/themes.md)
- Choose SQLite, PostgreSQL, MySQL, MariaDB, MongoDB, single-file JSON, or directory JSON. Each has a standalone Compose file. Platform-specific instructions cover repository builds, TLS routing, volumes, and secrets. [Deployment guide](docs/deployment.md)
- Automate with scoped bearer tokens, the CLI, WebMCP, or standalone MCP. [API and CLI guide](docs/interfaces.md)
- Review security controls, tests, and load measurements. [Security verification](docs/security.md) · [Performance](docs/performance.md)

## Local development

Use the Go toolchain specified in `go.mod` and Bun 1.4.4. The compiled React bundle is included so a Go build can run directly from a checkout.

```sh
bun install --frozen-lockfile
bun run check:i18n
bun run build
go build -o bin/th0ughts ./cmd/th0ughts
go test ./...
bin/th0ughts -listen localhost:8080 -data ./data serve
```

Set the master key before starting the server. Rebuild the binary after updating frontend assets. Run `bun run --bun playwright install chromium` and `bun run test:browser` for the mobile and desktop browser flows. Source code, comments, documentation, and test names are in English. Product text belongs in both language catalogs. [Contribution guide](CONTRIBUTING.md)
