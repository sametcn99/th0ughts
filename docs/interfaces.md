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
