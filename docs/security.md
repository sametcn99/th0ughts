# Security verification

The trust boundary is a single instance with one owner. Public readers can retrieve published material. Automation receives explicit token scopes. Theme authors supply data and presentation rules. The trusted Go process owns database access, object storage access, credential handling, and authenticated commands.

## Controls and verification evidence

The identifiers below refer to OWASP ASVS 5.0.0. The table records implementation and test evidence for the stated behavior. A full independent Level 2 assessment remains open. TLS termination, infrastructure access, backups, and log retention depend on the deployment. [ASVS standard and verification requirements](https://asvs.owasp.org/)

| ASVS requirements | Implementation | Verification |
| --- | --- | --- |
| V1.2.1, V1.3.1, V3.2.2 | Go escapes HTML attributes and text, encodes JSON script data with the standard JSON encoder, and sanitizes Markdown HTML with Bluemonday. React renders user text through its normal text API. | Review `server/page.go`, `server/post.go`, and `web/`. Public visibility tests inspect raw SSR and Markdown. |
| V1.2.4 | SQL uses bound values. MongoDB filters are structured BSON with fixed operator names. File keys have a constrained alphabet. | Driver contract and publication tests run against nine stores. |
| V1.3.2, V1.3.4 | Theme compiler emits recognized elements, slots, and CSS syntax. Active content and uploaded SVG are excluded. There is no template eval or user Go template execution. | `TestRestrictedTheme`, `TestCatalog`, and `FuzzCompile`. |
| V3.3.1–4, V3.4.1–5 | HTTPS deployments issue host-prefixed Secure, HttpOnly, SameSite cookies, HSTS, CSP, nosniff, and a same-origin referrer policy. Plain HTTP loopback supports development. Browser writes check origin, fetch metadata, and a custom request header for session cookies. | Inspect `server/app.go`. Test missing CSRF protection, owner and guest pages, and genuine browser login/logout. |
| V5.2.1–3, V5.3.1 | Media is bounded to 32 MiB and checked by content type. Images are checked by dimensions. Archive imports bound entry count, individual and total size, permit validated IDs, and write through the storage service. | S3 and SQLite-to-JSON backup round trips, archive fuzzing, draft media tests and restart recovery in `TestRestartRollsBackInterruptedRestore`. |
| V6.2.1, V6.2.7–10 | Passwords accept 12–256 bytes, use Argon2id with 64 MiB memory, three iterations, random salts, and constant-time comparison. Login uses standard password-manager-compatible inputs. | `TestTOTPAndRevocation`, browser login and source review. |
| V6.4, V6.5 | Passkeys require user verification; browser ceremonies are bound to an origin, a short-lived challenge, and the applicable session. Optional TOTP rejects replay; recovery code digests are single-use. Local master-key recovery resets account identity. | Desktop Chromium with a virtual CTAP2 authenticator registers and signs in. Go tests reject a repeated TOTP, a reused recovery code, and an old session after account recovery. |
| V7.2.1–4, V7.4.1, V8.2.1 | Random 256-bit reference tokens are verified on the server; grants store digests. Owner cookies last 12 hours. Sensitive operations require verification within ten minutes. Revocation applies to subsequent MCP calls too. | `TestCLIAndStdio`, `TestMCPBearerRevocation`, `TestTOTPAndRevocation`, and browser sign-out. |
| V11.3.1, V11.5.1 | AES-GCM protects database locators and S3/TOTP secrets. Each encryption uses a new random nonce and purpose-specific associated data. Operator-generated random master keys are required. | Review `feed/auth.go` and `server/app.go`. Keep the original key with offline infrastructure backups. |
| V14.3, V16.5.1 | Private, owner and preview responses bypass the public cache. The public cache varies by language, representation and URL and discards obsolete publication versions. HTTP errors use stable codes. | `TestPublicVisibility` alternates private/public reads and verifies post withdrawal. |
| V16.2, V16.3 | An allowlisted set of authentication, configuration and recovery operations emits structured events with status and duration. The logger uses credential type without including credential material, headers, request bodies, or connection strings. | Inspect `server/audit.go`; monitor application stderr at the deployment boundary. |

## Operational assumptions

- Terminate HTTPS at the platform proxy and keep database ports private. Set the public site URL and preserve the master key. Use separate storage credentials and buckets per installation.
- Keep Go, Bun build tooling, operating system images, and database services updated. CI runs Go vulnerability analysis, bun audit, the race detector, browser tests, and fuzz checks.
- Backups include private content and are delivered only to recently authenticated owners. Store archives with the same protection as the live database.
- Current evidence covers application behavior and local integration. It does not certify the underlying host, an external S3 provider, the operator's TLS configuration, or independent ASVS conformance.
