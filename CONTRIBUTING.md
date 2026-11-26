# Contributing

For a new theme, copy `themes/starter.html`, choose a unique ID, and edit that one file. The Go embed declaration discovers it automatically. Run `go test ./themes`, upload the file to a local instance, and preview English and Turkish at desktop and phone widths. [Theme format](docs/themes.md)

To share a theme directly with other owners, start a thread in Show and tell. Use a pull request if you would also like it bundled with th0ughts. [Share a theme](https://github.com/sametcn99/th0ughts/discussions/new?category=show-and-tell)

For a product change, keep public visibility and write authorization in the shared Go service. Include both translations of new product strings in `server/locales`. Use stable English identifiers in code and protocol payloads. A new language needs a catalog, supported locale registration, date and search behavior, language selector entries, and browser coverage. Include placeholders consistently across catalogs, and use plural rules when displaying counts.

Run `bun run check:i18n`, `bun run build`, `go test -race ./...`, and the affected browser tests. Commit the generated owner bundle together with its frontend source. Driver tests use disposable local databases named by the optional `TEST_postgres`, `TEST_mysql`, `TEST_mariadb`, and `TEST_mongodb` test variables. S3 tests use `TEST_S3`, optionally `TEST_S3_ACCESS` and `TEST_S3_SECRET`, and a disposable bucket named `thoughts-test`. These variables belong to the test process.

## Publishing a GitHub Release

The version embedded in the executable comes from `updates/VERSION`. Set it to a stable `vMAJOR.MINOR.PATCH`, update the package version in `package.json` to match, rebuild the embedded assets and merge the changes. From the verified commit, create and push the matching annotated tag.

```sh
version=$(cat updates/VERSION)
git tag -a "$version" -m "Release $version"
git push origin "refs/tags/$version"
```

**Build release** rebuilds the React assets, runs the Go race suite, cross-compiles standalone Linux amd64 and arm64 binaries, and generates `SHA256SUMS`. It creates a draft with generated release notes, uploads all assets, checks their GitHub-reported digests and then publishes. A rerun can resume an unfinished draft. Published tags and releases must remain immutable; ship corrections as a new version. Run the workflow manually on a branch to test packaging without publishing a tag or a release.

Keep the managed launch protocol compatible across versions. The existing launcher invokes the candidate with the original flags and the private `serve-worker` command. File descriptor 3 carries its newline-terminated version once the listener and store are ready, and exit status 75 requests an update restart.
