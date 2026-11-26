FROM oven/bun:1.4.4-alpine AS web
WORKDIR /src
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile
COPY web web
COPY server/locales server/locales
RUN bun run build

FROM golang:1.27.4-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/server/assets/owner.js /src/server/assets/thread.js server/assets/
COPY --from=web /src/server/preview server/preview
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/th0ughts ./cmd/th0ughts

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -g 65532 app && adduser -D -H -u 65532 -G app app && mkdir -p /data /run/db-init && chown app:app /data /run/db-init
COPY --from=build /out/th0ughts /usr/local/bin/th0ughts
USER app
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["th0ughts"]
CMD ["-data","/data","serve"]
