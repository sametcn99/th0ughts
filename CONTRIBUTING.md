# Contributing

For a new theme, copy `themes/starter.html`, choose a unique ID, and edit that one file. The Go embed declaration discovers it automatically. Run `go test ./themes`, upload the file to a local instance, and preview English and Turkish at desktop and phone widths. [Theme format](docs/themes.md)

For a product change, keep public visibility and write authorization in the shared Go service. Include both translations of new product strings in `server/locales`. Use stable English identifiers in code and protocol payloads. A new language needs a catalog, supported locale registration, date and search behavior, language selector entries, and browser coverage. Include placeholders consistently across catalogs, and use plural rules when displaying counts.

Run `bun run check:i18n`, `bun run build`, `go test -race ./...`, and the affected browser tests. Commit the generated owner bundle together with its frontend source. Driver tests use disposable local databases named by the optional `TEST_postgres`, `TEST_mysql`, `TEST_mariadb`, and `TEST_mongodb` test variables. S3 tests use `TEST_S3`, optionally `TEST_S3_ACCESS` and `TEST_S3_SECRET`, and a disposable bucket named `thoughts-test`. These variables belong to the test process.
