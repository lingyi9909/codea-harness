# Code Harness Web implementation status

Development branch: feature/code-harness-web-implementation
Draft PR: #69 (stacked on T01 PR #68).
Reference: PR #67 design and four approved screenshots.

This is real TypeScript source, not an offline HTML demo. The React pages call
/api/v1 and SSE only. Without native capability evidence, the server deliberately
shows Code Review as unavailable: it will never generate fake progress or a
sample report. Do not bypass this gate to make a demo appear successful.

From web-workbench:
- npm ci --offline --ignore-scripts --no-audit --no-fund
- npm run gen:api
- npm run typecheck
- npm run build:web
- npm test
- npm run migrate (only on an isolated *_dev or *_test MySQL 8.4 database)
- npm start (Fastify binds exclusively to 127.0.0.1:4176)

Configuration (environment variables):
- HARNESS_MYSQL_HOST, HARNESS_MYSQL_PORT, HARNESS_MYSQL_DATABASE
- HARNESS_MYSQL_USER, HARNESS_MYSQL_PASSWORD (DML application account)
- HARNESS_MYSQL_MIGRATION_USER, HARNESS_MYSQL_MIGRATION_PASSWORD (DDL only)
- HARNESS_MIGRATION_APPROVED_DATABASE must exactly match database and end with _dev/_test
- HARNESS_REPOSITORY_ROOTS: semicolon-separated approved local canonical roots
- HARNESS_DATA_DIR, HARNESS_HTTP_PORT, HARNESS_ALLOWED_ORIGIN

MySQL migrations run separately; normal server does not silently grant DDL
rights. The first app startup prints a short-lived pairing code in the local
terminal; do not put it in logs or URLs.

Testing boundaries:
- GitHub Actions Windows confirms typecheck, frontend production bundle and
  isolated local tests. It does not substitute actual company Windows OpenCode
  native APIs, model, permissions, durable event behavior, MySQL 8.4,
  offline installation, restart/recovery or visual acceptance.
- Native execution continues disabled until evidence-backed T01 completes.
- Tasks and reports are never produced by an in-memory fake OpenCode service.
