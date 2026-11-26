# th0ughts

A single-owner microblog with two pages. Readers open the timeline at `/`. The owner opens `/login` directly, signs in, and writes, translates, uploads media, changes themes, and manages the site on the same timeline.

Go serves the initial HTML and the embedded React application. Reading, searching, pagination, per-post links, RSS, and Markdown work without JavaScript. The timeline loads 20 posts at a time and appends the next page as you scroll. One server process owns the data store. On managed installations, a launcher restarts the server during updates.

## Install on a VPS

Run the interactive wizard in a Bash terminal on the server.

```sh
curl -fsSL https://sametcn99.github.io/th0ughts/install.sh | bash
```

Choose English or Turkish, a database, host port, binding, directory and Compose project. The wizard generates a private master key and waits for a healthy installation. [Documentation and screenshot gallery](https://sametcn99.github.io/th0ughts/) · [Inspect the installer](deploy/install.sh)

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

Write directly in the timeline and select **Publish** or **Save draft**. Short posts appear in full. A title or more than 280 visible characters gives the post an article preview in the same timeline. Its permanent link opens the complete text and attachments. Use **Post options** to add a title, tags, media or a translation. English and Turkish tabs edit the same post. **Original language** controls the fallback when a translation has not been published. The original must be published for the post to appear publicly. **Edit** opens its current revision. Concurrent edits return a conflict so another client's changes survive.

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

- Check **Settings → Updates** for stable GitHub releases. The Linux launcher can install a verified update and restart the server from this panel. [Updating an installation](docs/deployment.md#application-updates) · [GitHub Releases](https://github.com/sametcn99/th0ughts/releases)
- Send [signed comment webhooks](docs/interfaces.md#signed-comment-webhooks) to your own HTTPS receiver when guests comment or the owner approves or deletes a comment.
- Enable guest discussions in **Settings → Comments**. Readers can reply with text, Markdown, and images with a required, DNS-checked email address. Inbox codes are optional. The owner can review submissions, delete comments, or lock individual threads. [Comment verification settings](docs/deployment.md#guest-comments-and-verification-mail)
- Upload a single HTML file from **Settings → Appearance**. Preview it, select its options, and save. Browse community themes and leave feedback in Discussions, or start a thread for your own theme. [Community themes](https://github.com/sametcn99/th0ughts/discussions/categories/show-and-tell) · [Share a theme](https://github.com/sametcn99/th0ughts/discussions/new?category=show-and-tell) · [Theme authoring guide](docs/themes.md)
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

## Documentation site and screenshots

The documentation opens in [English](https://sametcn99.github.io/th0ughts/). Select [Türkçe](https://sametcn99.github.io/th0ughts/tr/) for Turkish.

Every Markdown file under `docs/` is published in the [documentation index](https://sametcn99.github.io/th0ughts/docs/). Article pages are generated from the repository at build time and can be read with JavaScript disabled.

Run `bun run build:pages` to render the English and Turkish React site into `pages/dist`. After building `bin/th0ughts` and installing Playwright Chromium, run `bun run screenshots` to refresh the images in `pages/public/screenshots`, their manifest and the gallery below. Capture uses a disposable SQLite instance with sample content. GitHub Actions captures fresh images and publishes the site on each push to main.

<!-- gallery:start -->
<details>
<summary>Application screenshots (57)</summary>

Build the Go binary (see Local development), install Chromium with `bun run --bun playwright install chromium`, and run `bun run screenshots` to refresh this gallery. The script creates and removes its own demo instance.

**The public timeline**

![The public timeline](https://sametcn99.github.io/th0ughts/screenshots/timeline.webp)

**Starting an empty timeline**

![Starting an empty timeline](https://sametcn99.github.io/th0ughts/screenshots/empty.webp)

**First sign-in with the master key**

![First sign-in with the master key](https://sametcn99.github.io/th0ughts/screenshots/bootstrap.webp)

**Owner and database setup**

![Owner and database setup](https://sametcn99.github.io/th0ughts/screenshots/setup.webp)

**Connecting an external database**

![Connecting an external database](https://sametcn99.github.io/th0ughts/screenshots/setup-database.webp)

**Password, authenticator and passkey login**

![Password, authenticator and passkey login](https://sametcn99.github.io/th0ughts/screenshots/login.webp)

**Choosing the interface language**

![Choosing the interface language](https://sametcn99.github.io/th0ughts/screenshots/language.webp)

**A short post at its permanent link**

![A short post at its permanent link](https://sametcn99.github.io/th0ughts/screenshots/short-post.webp)

**An article opened from the timeline**

![An article opened from the timeline](https://sametcn99.github.io/th0ughts/screenshots/post.webp)

**Search results**

![Search results](https://sametcn99.github.io/th0ughts/screenshots/search.webp)

**Search without matches**

![Search without matches](https://sametcn99.github.io/th0ughts/screenshots/search-empty.webp)

**Posts with the studio tag**

![Posts with the studio tag](https://sametcn99.github.io/th0ughts/screenshots/tag.webp)

**Older posts**

![Older posts](https://sametcn99.github.io/th0ughts/screenshots/archive.webp)

**The owner's timeline**

![The owner's timeline](https://sametcn99.github.io/th0ughts/screenshots/owner.webp)

**Writing a post**

![Writing a post](https://sametcn99.github.io/th0ughts/screenshots/compose.webp)

**Editing attachments and sharing metadata**

![Editing attachments and sharing metadata](https://sametcn99.github.io/th0ughts/screenshots/edit-media.webp)

**Editing a Turkish translation**

![Editing a Turkish translation](https://sametcn99.github.io/th0ughts/screenshots/translation.webp)

**A private draft**

![A private draft](https://sametcn99.github.io/th0ughts/screenshots/draft.webp)

**Site profile and language settings**

![Site profile and language settings](https://sametcn99.github.io/th0ughts/screenshots/settings.webp)

**Application updates**

![Application updates](https://sametcn99.github.io/th0ughts/screenshots/updates.webp)

**Theme, typography and spacing**

![Theme, typography and spacing](https://sametcn99.github.io/th0ughts/screenshots/appearance.webp)

**Theme preview with published posts**

![Theme preview with published posts](https://sametcn99.github.io/th0ughts/screenshots/preview.webp)

**Sample content in owner preview**

![Sample content in owner preview](https://sametcn99.github.io/th0ughts/screenshots/preview-owner.webp)

**Short post permalink preview**

![Short post permalink preview](https://sametcn99.github.io/th0ughts/screenshots/preview-short.webp)

**Article card preview**

![Article card preview](https://sametcn99.github.io/th0ughts/screenshots/preview-article-list.webp)

**Full Article preview**

![Full Article preview](https://sametcn99.github.io/th0ughts/screenshots/preview-article.webp)

**Sign-in preview**

![Sign-in preview](https://sametcn99.github.io/th0ughts/screenshots/preview-login.webp)

**Composer preview**

![Composer preview](https://sametcn99.github.io/th0ughts/screenshots/preview-composer.webp)

**Settings dialog preview**

![Settings dialog preview](https://sametcn99.github.io/th0ughts/screenshots/preview-settings.webp)

**Theme downloads and revision history**

![Theme downloads and revision history](https://sametcn99.github.io/th0ughts/screenshots/theme-history.webp)

**Scoped access token settings**

![Scoped access token settings](https://sametcn99.github.io/th0ughts/screenshots/tokens.webp)

**Issued token and revocation**

![Issued token and revocation](https://sametcn99.github.io/th0ughts/screenshots/token-issued.webp)

**Local storage, backup and restore**

![Local storage, backup and restore](https://sametcn99.github.io/th0ughts/screenshots/storage.webp)

**S3-compatible storage**

![S3-compatible storage](https://sametcn99.github.io/th0ughts/screenshots/storage-s3.webp)

**Passkey and authenticator settings**

![Passkey and authenticator settings](https://sametcn99.github.io/th0ughts/screenshots/security.webp)

**Verify identity for sensitive actions**

![Verify identity for sensitive actions](https://sametcn99.github.io/th0ughts/screenshots/reauth.webp)

**Authenticator enrollment**

![Authenticator enrollment](https://sametcn99.github.io/th0ughts/screenshots/authenticator.webp)

**Single-use recovery codes**

![Single-use recovery codes](https://sametcn99.github.io/th0ughts/screenshots/recovery-codes.webp)

**Contact Sheet theme**

![Contact Sheet theme](https://sametcn99.github.io/th0ughts/screenshots/theme-contact-sheet.webp)

**Dusk theme**

![Dusk theme](https://sametcn99.github.io/th0ughts/screenshots/theme-dusk.webp)

**Index theme**

![Index theme](https://sametcn99.github.io/th0ughts/screenshots/theme-index.webp)

**Lagoon theme**

![Lagoon theme](https://sametcn99.github.io/th0ughts/screenshots/theme-lagoon.webp)

**Postmark theme**

![Postmark theme](https://sametcn99.github.io/th0ughts/screenshots/theme-postmark.webp)

**Risograph theme**

![Risograph theme](https://sametcn99.github.io/th0ughts/screenshots/theme-risograph.webp)

**Starter theme**

![Starter theme](https://sametcn99.github.io/th0ughts/screenshots/theme-starter.webp)

**Switchyard theme**

![Switchyard theme](https://sametcn99.github.io/th0ughts/screenshots/theme-switchyard.webp)

**Mobile timeline in Turkish**

![Mobile timeline in Turkish](https://sametcn99.github.io/th0ughts/screenshots/mobile.webp)

**A short post on mobile**

![A short post on mobile](https://sametcn99.github.io/th0ughts/screenshots/mobile-short-post.webp)

**Translated post on mobile**

![Translated post on mobile](https://sametcn99.github.io/th0ughts/screenshots/mobile-post.webp)

**Mobile sign-in in Turkish**

![Mobile sign-in in Turkish](https://sametcn99.github.io/th0ughts/screenshots/mobile-login.webp)

**Appearance settings on mobile**

![Appearance settings on mobile](https://sametcn99.github.io/th0ughts/screenshots/mobile-appearance.webp)

**Threaded comments and images**

![Threaded comments and images](https://sametcn99.github.io/th0ughts/screenshots/comment-thread.webp)

**Reading replies on mobile**

![Reading replies on mobile](https://sametcn99.github.io/th0ughts/screenshots/comment-mobile.webp)

**Guest comment form**

![Guest comment form](https://sametcn99.github.io/th0ughts/screenshots/guest-comment.webp)

**Guest comment verification policy**

![Guest comment verification policy](https://sametcn99.github.io/th0ughts/screenshots/comment-verification.webp)

**Signed comment webhook settings**

![Signed comment webhook settings](https://sametcn99.github.io/th0ughts/screenshots/comment-webhook.webp)

**Comment settings and moderation queue**

![Comment settings and moderation queue](https://sametcn99.github.io/th0ughts/screenshots/comment-moderation.webp)

</details>
<!-- gallery:end -->
