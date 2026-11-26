FROM oven/bun:1.4.4-alpine AS web
WORKDIR /src
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile
COPY web web
COPY server/locales server/locales
RUN bun run build

FROM rust:1.100-bookworm AS kv
WORKDIR /src
COPY native native
COPY scripts/build-native.sh scripts/build-native.sh
RUN scripts/build-native.sh

FROM golang:1.27.4-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/server/assets/owner.js /src/server/assets/thread.js server/assets/
COPY --from=kv /src/native/lib native/lib
COPY --from=kv /src/storage/nativekv_build.h storage/nativekv_build.h
COPY --from=web /src/server/vendor server/vendor
COPY --from=web /src/server/preview server/preview
RUN CGO_ENABLED=1 go build -tags "nativekv,netgo,osusergo" -trimpath -ldflags='-s -w -linkmode external -extldflags "-static"' -o /out/th0ughts ./cmd/th0ughts

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -g 65532 app && adduser -D -H -u 65532 -G app app && mkdir -p /data /run/db-init && chown app:app /data /run/db-init
COPY --from=build /out/th0ughts /usr/local/bin/th0ughts
USER app
RUN th0ughts version
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["th0ughts","-managed-updates"]
CMD ["-data","/data","serve"]
