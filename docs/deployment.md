# Deployment

Each Compose file is standalone. Run one application process per site and keep its data volume. Master keys must contain at least 32 bytes. Generate one random key per installation and keep a protected copy separately. The server uses it to encrypt the first database connection, TOTP secrets, and S3 credentials. An owner password, passkeys, sessions, and scoped automation tokens handle routine access.

| Installation | Compose file | Persistent content |
| --- | --- | --- |
| SQLite | `deploy/compose.sqlite.yaml` | Application volume, SQLite database in WAL mode |
| Single JSON | `deploy/compose.json.yaml` | Application volume and one JSON data file |
| Directory JSON | `deploy/compose.json-dir.yaml` | Application volume and one file per record |
| PostgreSQL | `deploy/compose.postgres.yaml` | Application, database, and credential volumes |
| MySQL | `deploy/compose.mysql.yaml` | Application, database, and credential volumes |
| MariaDB | `deploy/compose.mariadb.yaml` | Application, database, and credential volumes |
| MongoDB | `deploy/compose.mongodb.yaml` | Application, database, and credential volumes |
| Coolify | `deploy/compose.coolify.yaml` | Application volume, SQLite by default |
| Dokploy | `deploy/compose.dokploy.yaml` | Application volume, SQLite by default |

The database variants create their private credentials in a one-shot setup service. Database ports stay inside the Compose network. The ordinary host variants bind the web port to loopback. Set the public URL in site settings after configuring your HTTPS proxy. For an existing external database, select the driver on first setup and enter the connection there. PostgreSQL accepts a connection URI, MySQL and MariaDB a Go MySQL DSN, and MongoDB a URI with a database path.

All seven drivers passed a native-process check through the compiled CLI and HTTP server. Each run initialized an owner, published a translated post, exported a backup, restarted the server, reused its owner session, and verified the public page excluded the private translation. The separate container smoke workflow builds each database recipe and exercises a restart through HTTP.

## Interactive VPS installation

Open a terminal on the Linux host and run:

```bash
curl -fsSL https://sametcn99.github.io/th0ughts/install.sh | bash
```

The published script fetches the Git revision used to build the documentation. To inspect it first, download it with `curl -fsSLo install.sh https://sametcn99.github.io/th0ughts/install.sh`, read the file, then run `bash install.sh`. From a reviewed local checkout, `bash deploy/install.sh --source "$PWD"` installs its latest committed revision.

Choose a language, database, host port, bind address, new directory and unique Compose project name. Loopback is the default for an SSH tunnel or an HTTPS proxy. Binding to 0.0.0.0 publishes plain HTTP to the network. Use HTTPS for remote owner access. The wizard uses a new directory and checks for existing Compose resources before provisioning. It can install missing prerequisites on Ubuntu or Debian after confirmation. Other Linux distributions need Bash, Git, curl, OpenSSL, Docker Engine and its Compose plugin.

After the health check, follow the displayed SSH tunnel instructions or visit your HTTPS domain at `/login`. The master key is in the installation's mode-0600 `.env`. Keep this file and the Docker volumes when making a full backup. From the installation directory, run `./manage ps` for status, `./manage logs --tail=100 app` for diagnostics, or `./manage up -d --build --wait` to resume an interrupted build. `./manage down` stops the project while retaining volumes. For the first upgrade from a version without **Settings → Updates**, fetch and check out a reviewed revision in `source/`, then run `./manage up -d --build --wait` with the existing data volume and master key.

## Application updates

Open **Settings → Updates** and select **Check for updates**. The server checks the latest stable GitHub Release and caches a successful check for five minutes. The panel shows the running version even when GitHub is unavailable. A new repository can have a running development baseline before the first release is published.

On Linux amd64 and arm64, the supplied Docker image runs the update launcher by default. This includes the VPS wizard and the Compose recipes for Coolify and Dokploy. With a recently verified owner session, **Install … and restart** downloads the selected executable, verifies its byte count and SHA-256 against GitHub's release metadata, drains HTTP requests and restarts the application. The launcher promotes the candidate after it opens the store, binds the port and reports the expected version. If startup fails, it tries the previous executable. The owner panel reconnects and checks the running version. Keep a current backup before updating across database changes. Startup fallback covers the executable; restoring an older data schema requires a compatible backup.

The selected executable and its authenticated metadata live under `updates/` in the persistent data directory. The master key and volume must survive recreation. The volume must allow execution for the application user. Container updates run with the existing non-root user and read-only image filesystem. Base image packages, database services and Compose configuration continue to follow your normal deployment process. A newly deployed image at the same or a newer version takes precedence over the cached release.

For a standalone Linux release, run `./th0ughts-linux-amd64 -managed-updates -data ./data serve` (use the arm64 executable on an arm64 system). Keep the same startup command after updates. Starting with ordinary `serve` retains manual deployment and the in-app version check. To recover using a specific bundled version, stop the launcher and start that binary with ordinary `serve` against a compatible store. [Release downloads](https://github.com/sametcn99/th0ughts/releases)

## Coolify

Use a Git-backed **Application** with the **Docker Compose** build strategy. Select the public Git repository as the source. Set **Base Directory** to `/deploy` and **Docker Compose Location** to `/compose.coolify.yaml`. Coolify resolves the file's `build: ..` from that base directory, reaching the root Dockerfile and frontend sources. Set `TH0UGHTS_MASTER_KEY` in the resource environment. Assign the HTTPS domain to service `app`, internal port **8080**. For the domain field that accepts a port suffix, enter `https://feed.example.com:8080`. In the application's site settings use the browser-facing URL `https://feed.example.com`. [Coolify Compose build and routing](https://coolify.io/docs/applications/builds/docker-compose)

## Dokploy

Create a **Docker Compose** service, connect the Git repository, and set its Compose path to `deploy/compose.dokploy.yaml`. Source builds require Compose mode. Add `TH0UGHTS_MASTER_KEY` in **Environment**. The Compose service explicitly passes this variable into the app container. Under **Domains → Add Domain**, select `app`, the public hostname, container port **8080**, and HTTPS. Preview Compose to inspect the platform-generated proxy configuration. [Dokploy Compose configuration](https://docs.dokploy.com/docs/core/docker-compose) · [Dokploy domain routing](https://docs.dokploy.com/docs/core/docker-compose/domains)

Both platform files expose the container port internally, use `/healthz`, and mount a named volume at `/data`. The deployment check validates Compose's resolved build context, HTTP port, required environment, and volume. The platform instructions were reviewed against the linked documentation. A public-domain deployment has not been exercised on a connected Coolify or Dokploy instance.

## Backup, migration, and recovery

- Download a ZIP under **Settings → Storage** with a recently verified owner session. The archive contains posts, translations, media bytes, site preferences, and custom themes. It contains private drafts. Store it accordingly.
- Set up a fresh target on the desired driver, configure disk or S3, and restore the ZIP. A restore requires an empty content store and keeps the target's credentials. Post and attachment IDs survive migration. The restore journal rolls back an interrupted import the next time the application opens.
- To preserve sessions, passkeys, and encrypted connection details in a full infrastructure backup, retain the database, all volumes, any S3 objects, and the original master key.
- Stop the serving process before offline maintenance. With the same data directory and master key, run `th0ughts -data /data recover` and supply the replacement password on standard input. Recovery invalidates old sessions and API tokens. Run `th0ughts -data /data theme-reset` to restore the built-in Paper theme.
- Keep the Compose project name stable when upgrading. Rebuild, deploy, and check `/healthz`. Retain the existing volume and master key.

## Guest comments and verification mail

Enable guest comments in **Settings → Comments**. Choose **Review each comment before publishing** or **Publish immediately** and save. Both modes require a syntactically valid email address and a domain with a mail route in DNS. The server checks for MX records, rejects a null MX and falls back to A/AAAA when MX records are absent. A temporary DNS failure asks the guest to retry. DNS checks establish domain-level routing. Mailbox ownership requires an inbox code. [SMTP mail routing](https://www.rfc-editor.org/rfc/rfc5321#section-5.1) · [Null MX](https://www.rfc-editor.org/rfc/rfc7505)

**Require a code sent by email** is off by default, including when upgrading from a version without this setting. To enable it, open **Verification email delivery** and configure a trusted SMTP relay using STARTTLS (typically port 587) or implicit TLS (typically 465). Save the relay, enable the checkbox and save the comment settings. The master key encrypts relay credentials. Configure the relay again after restoring a portable backup to a new instance.

Set the image limit to zero to disable uploads and use the Markdown switch to control formatting. Switching guest comments off hides all threads and their images from readers. Closing an individual post keeps approved conversations readable.

Display names are optional. Email addresses and codes are excluded from published comments and portable backups. Each comment accepts up to 6,000 characters and up to four JPEG, PNG, or WebP images of 4 MiB each, subject to the owner's settings. The server decodes images and re-encodes them as JPEG before storing them on disk or S3.

Use the moderation queue to approve or delete submissions. Deleting a published comment removes its content and images while keeping replies in context. The moderation interface does not offer text editing. Backups preserve the tree, attachments, pending submissions, moderation states, and post locks.

To notify another service when guests comment or you moderate a thread, open **Signed comment webhook** in the same settings panel. Configure one trusted HTTPS receiver and signing secret per site. The panel shows the time and HTTP result of the most recent attempt. [Events, retry schedule and receiver verification example](interfaces.md#signed-comment-webhooks)
