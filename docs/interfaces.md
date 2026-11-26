# HTTP, CLI, and agents

Retrieve the live API description at `/api/v1/openapi.json`. Public requests read published translations. An owner session or bearer token with `posts:read` can explicitly request private data. Writes require the corresponding scope. Cookie-authenticated writes also require `X-Th0ughts-Request: 1`.

## REST

```sh
curl 'http://localhost:8080/api/v1/posts?lang=tr&limit=20'
curl 'http://localhost:8080/feed.md?lang=en'
curl -H 'Accept: text/markdown' 'http://localhost:8080/?lang=tr'
```

Create with `POST /api/v1/posts`, update the complete record with `PUT /api/v1/posts/<id>`, and delete with `DELETE /api/v1/posts/<id>`. Supply the current revision when editing. The response to a list contains `posts` and an optional opaque `next` cursor. Include the same search, language, and tag filters when requesting the next page.

The named command endpoints under `/api/v1/<command>` accept JSON objects. Discover permitted commands and their input schemas with `GET /api/v1/tools`. Upload a media file as a raw request body to `POST /api/v1/upload`, then include its ID in the post's `media` and translation's `alt` dictionary. A new attachment remains private until a published post references it.

## Remote CLI

Global flags precede the command. Pass JSON through standard input or as a single argument. The CLI checks command errors through its exit status. `-lang tr -h` displays Turkish help. `-json` produces compact output and retains stable protocol names.

```sh
th0ughts -url https://feed.example.com -lang tr -json list_posts '{"lang":"tr","limit":5}'
th0ughts -url https://feed.example.com -credentials ./writer.token save_post \
  '{"original":"en","text":{"en":{"body":"A new thought.","published":false}}}'
th0ughts -url https://feed.example.com -credentials ./writer.token -file ./photo.jpg upload
```

Create a token in **Settings → Access tokens** and keep the returned value in a permissions-restricted file. Choose individual scopes and an expiry. For owner-only maintenance, pipe a JSON object containing `password` and optional `code` into `th0ughts -url <site> -credentials <session-file> login`. Keep secrets in protected files or standard input. A fresh owner session can download and restore backups with `-file <archive> backup` or `restore`. For an empty site, the `setup` command accepts `master`, `password`, and optional `driver` and `address` on standard input.

## MCP

- **Browser WebMCP** registers tools with `document.modelContext` when provided by the browser. The page registers names, localized descriptions, and input schemas for the active session. Browsers can read the public feed through HTML, Markdown, or REST independently of that capability.
- **Streamable HTTP** is served at `/mcp`. Supply the normal bearer authorization header and use `?lang=tr` for Turkish tool descriptions. Each request resolves its principal again.
- **stdio** is available through `th0ughts -url <site> -credentials <token-file> -lang en mcp`. The process first discovers authorized tools and forwards calls to the remote API. Standard output is reserved for the MCP protocol.

## Go packages

Embed the same publishing logic with `storage.Open`, `feed.New`, `feed.NewAuth`, `feed.NewThemes`, and `feed.Library`. Use `feed.Actor{}` for a visitor and obtain request actors through `Auth.Resolve` after transport authentication. All supplied Go actors are trusted inputs from the embedding application. Never deserialize an actor from a client payload.

Create a service once with `feed.New(store)` at startup and pass it to the code that needs it. This example reads the public timeline in the requested language.

```go
package reader

import (
    "context"

    "github.com/sametcn99/th0ughts/feed"
)

func Latest(ctx context.Context, service *feed.Service, lang string) (feed.Page, error) {
    return service.List(ctx, feed.Actor{}, feed.Filter{Lang: lang, Limit: 20})
}
```

Reuse one service across requests to share its search index, mutation lock, and cache generation. A single writer process owns each site. Resolve credentials at your transport boundary before calling the service to publish or manage settings.

A new store implements `storage.Store` and its public ordering, cursor, missing-record and durability contract. Pass it to the service directly or register its built-in driver and deployment recipe when contributing it to the project. The local JSON drivers additionally hold an exclusive file lock.

### Theme commands

Use the shared `service.Themes` catalog when embedding the service. `publish_theme` takes `{"settings":{...},"source":"..."}` and commits both changes together. Publishing a file requires both the `settings` and `themes` scopes. `save_settings` validates option overrides against the installed schemas. `get_theme` returns the current HTML in `source`, the last revision in `previous`, and older revisions in `history`. Pass one of these sources to `install_theme` to restore it. `delete_theme` removes an inactive custom theme by ID. These commands are available through the named HTTP endpoints, CLI and agent tools.

### Guest comments

Read the first 20 root comments with `list_comments` and a `post` ID. Pass a `parent` comment ID to read a branch or the returned `next` value as `cursor` to continue. Each item includes `hasReplies`. The `comment_context` command accepts an `id` and returns its visible ancestors and the selected comment. A link to a specific reply uses `/?post=POST_ID#comment-COMMENT_ID`.

Read `comments.requireEmailCode` from `get_settings` to choose the submission flow. By default, submit a multipart form directly to `POST /api/v1/comments/POST_ID` with the required `email` field. The server validates the address syntax and DNS mail route before recording the comment. An invalid address returns HTTP 400; a temporary DNS failure returns HTTP 503 and the guest can retry.

When `requireEmailCode` is true, send `post`, `email` and optional `lang` to `POST /api/v1/comment-code` first. After DNS validation, the guest receives a six-digit code and the response contains a challenge ID. Submit the same multipart form with the `X-Comment-Challenge` and `X-Comment-Code` headers and omit the email field. The challenge is bound to the post, expires after ten minutes and is consumed on success.

Both flows accept `name`, `body`, `format` (`text` or `markdown`), optional `parent`, and repeated `image` file parts. The response status is `pending` or `approved`.

Owners use `pending_comments` with the `pending` or `published` status filter, `moderate_comment` with an `id` and an `approve` or `delete` action, and `set_thread_lock` with a post `id` and a `closed` boolean. These commands use the same permission checks across HTTP, CLI, and MCP.

### Signed comment webhooks

Under **Settings → Comments → Signed comment webhook**, enter your receiver's HTTPS URL and a shared signing secret (generate one with `openssl rand -hex 32`). Enable **Send comment events** and save. A recently authenticated owner can also call `configure_comment_webhook` with `enabled`, `url`, and `secret`. An empty secret keeps the current key for the same URL. `get_comment_webhook` returns the URL, enabled state, key presence and most recent delivery result. The master key encrypts the configuration in storage. Portable backups exclude both the configuration and the delivery queue.

| Event | When it is queued |
| --- | --- |
| `comment.created` | A guest submits a root comment or reply. The initial status is `pending` or `approved`. |
| `comment.approved` | The owner approves a pending comment. |
| `comment.deleted` | The owner deletes a comment. The payload retains thread identifiers and timestamps and omits the deleted content. |

The JSON envelope contains a stable event `id`, `type`, UTC `occurred_at` timestamp and a `data` object with the comment snapshot. Use `data.post`, `data.id` and the optional `data.parent` to locate a comment within a thread. Created and approved events can include the display name, text, format and image IDs. Only configure a receiver trusted to handle pending submissions. Email addresses and verification codes are excluded. Post removal, backup import and automatic pruning of deleted ancestors do not generate moderation events.

The server sends an HTTPS POST with `Content-Type: application/json`. Verify the signature on the original request bytes before parsing the JSON. This Python example takes the shared secret as a string, the raw body as bytes and the HTTP headers as a case-insensitive mapping.

```python
import hashlib
import hmac
import json
import time

def verify_comment_event(secret, headers, body):
    timestamp = headers.get("X-Th0ughts-Timestamp", "")
    if not (timestamp.isascii() and timestamp.isdecimal() and len(timestamp) <= 12):
        raise ValueError("Invalid delivery timestamp")
    if abs(time.time() - int(timestamp)) > 300:
        raise ValueError("Expired delivery")
    message = timestamp.encode("ascii") + b"." + body
    expected = "v1=" + hmac.new(secret.encode("utf-8"), message, hashlib.sha256).hexdigest()
    if not hmac.compare_digest(expected.encode(), headers.get("X-Th0ughts-Signature", "").encode()):
        raise ValueError("Invalid delivery signature")
    return json.loads(body)
```

Record the event ID and the corresponding work atomically before returning a 2xx response. Repeated deliveries carry the same ID and body, and each attempt is signed with its current Unix timestamp. Failed requests, including redirects, are retried after 10 seconds, 1 minute, 5 minutes, 30 minutes, 2 hours, 6 hours and 12 hours. Delivery stops after eight attempts. The queue survives server restarts. Events can arrive out of order when a delivery is retried. Disabling the webhook or changing its URL invalidates queued deliveries to the old destination. Rotating the secret for the same destination signs subsequent attempts with the new key.
