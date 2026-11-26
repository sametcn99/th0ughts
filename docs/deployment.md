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

After the health check, follow the displayed SSH tunnel instructions or visit your HTTPS domain at `/login`. The master key is in the installation's mode-0600 `.env`. Keep this file and the Docker volumes when making a full backup. From the installation directory, run `./manage ps` for status, `./manage logs --tail=100 app` for diagnostics, or `./manage up -d --build --wait` to resume an interrupted build. `./manage down` stops the project while retaining volumes. For upgrades, fetch and check out a reviewed revision in `source/`, then rebuild using the same launcher.

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

Comments are off on new and upgraded sites. Open **Settings → Comments → Verification email delivery** and enter the SMTP host, port, sender, and any required credentials. Choose STARTTLS (typically port 587) or implicit TLS (typically 465). The relay must accept mail from your instance and present a certificate trusted by the server. The master key encrypts relay credentials in the database. Restoring a portable backup onto a new instance requires configuring its relay again.

Choose **Review each comment before publishing** or **Publish after email verification**, then save. Set the image limit to zero to disable uploads and use the Markdown switch to control formatting. Switching guest comments off hides all threads and their images from readers. Closing an individual post keeps approved conversations readable.

Guests receive a one-use mailbox code for each submission. Display names are optional. Email addresses and codes are excluded from published comments and portable backups. Each comment accepts up to 6,000 characters and up to four JPEG, PNG, or WebP images of 4 MiB each, subject to the owner's settings. The server decodes images and re-encodes them as JPEG before storing them on disk or S3.

Use the moderation queue to approve or delete submissions. Deleting a published comment removes its content and images while keeping replies in context. The moderation interface does not offer text editing. Backups preserve the tree, attachments, pending submissions, moderation states, and post locks.

To notify another service when guests comment or you moderate a thread, open **Signed comment webhook** in the same settings panel. Configure one trusted HTTPS receiver and signing secret per site. The panel shows the time and HTTP result of the most recent attempt. [Events, retry schedule and receiver verification example](interfaces.md#signed-comment-webhooks)
