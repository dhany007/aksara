# Tech Plan - EPUB Translator Revamp

## Memory Context
- No repository directories matching `*-memory-system` were found.
- Read `README.md` and `prompt.md`; the current product is a self-hosted AI e-reader with PDF upload, DeepSeek translation, SQLite caching, server-rendered HTML reader, shelves, login, and reading progress.
- Read the current backend entrypoint and modules:
  - `cmd/server/main.go`
  - `internal/config/config.go`
  - `internal/db/db.go`
  - `internal/service/book.go`
  - `internal/service/translator.go`
  - `internal/worker/pipeline.go`
  - `internal/handler/*.go`
  - `internal/model/*.go`
  - `parser/extract.py`
  - `Dockerfile`, `docker-compose.yml`, `.env.example`, `Makefile`, `.gitignore`
- Existing conventions to keep where useful:
  - Go owns orchestration and calls Python as a subprocess for PDF extraction.
  - DeepSeek is used through an OpenAI-compatible HTTP chat completion API.
  - Keep the architecture simple and solo-developer friendly.
  - Preserve code blocks and common software engineering terms during translation.

## Requirement
- Revamp Aksara from a web e-reader into a local book translation tool.
- Remove PDF upload entirely.
- Read source PDFs from a root-level `books/` directory.
- Translate each book to natural Indonesian.
- Write one `.epub` file per source PDF, preferably under root-level `results/`.
- Remove the HTML reader, HTML templates, browser UI, auth flow, shelves, reading progress, and upload/status API.
- Keep the project focused on "translate books" only.

Assumptions:
- `books/` is the default input directory and `results/` is the default output directory.
- For MVP, scan direct `*.pdf` files in `books/`; recursive scanning can be added behind a flag if desired.
- If `results/<book>.epub` already exists, skip it by default to avoid accidental API cost; add an explicit overwrite option.
- "Hapus semua html" means remove the web app/templates and avoid standalone HTML output. EPUB itself requires internal XHTML files, so the builder will generate XHTML only inside the `.epub` package.
- Existing `data/` and `storage/` runtime folders should not be deleted automatically. They become obsolete and can be cleaned up manually after the user confirms they are no longer needed.

Non-goals:
- No browser reader.
- No login/session auth.
- No upload endpoint.
- No SQLite library/progress database.
- No OCR for scanned PDFs.
- No multi-user behavior.
- No EPUB editing UI.

## Backend Scope
- Replace the Echo HTTP server with a CLI or one-shot job entrypoint.
- Keep Go as the main application language.
- Keep Python/PyMuPDF for PDF extraction.
- Keep DeepSeek translation.
- Add EPUB generation.
- Remove backend code that only exists for the old web reader:
  - `internal/handler`
  - `internal/middleware`
  - `internal/db`
  - old reader/upload-oriented `internal/model` structs
  - upload/storage methods in `internal/service/book.go`
- Delete `web/templates/*.html` during implementation because the user explicitly asked to remove the HTML app.
- Update Docker and Compose so the container runs the translator job, mounts `books/` and `results/`, and exposes no HTTP port.

## Codebase Context
- `cmd/server/main.go` currently wires Echo, templates, auth middleware, SQLite, handlers, and background worker. This should be replaced or superseded by a command entrypoint such as `cmd/aksara/main.go`.
- `internal/config/config.go` currently requires web-specific env vars: `SESSION_SECRET`, `ADMIN_USERNAME`, `ADMIN_PASSWORD_HASH`, `PORT`, `DATA_DIR`, and `STORAGE_DIR`. These should be removed or replaced with translator-specific settings.
- `internal/worker/pipeline.go` currently depends on SQLite book/page rows and writes translated HTML fragments. The new pipeline should be filesystem-driven and write EPUB output atomically.
- `internal/service/translator.go` is reusable but its prompt and output contract should change. The current prompt asks for HTML; the new prompt should produce structured translated content that the EPUB builder converts to XHTML internally.
- `parser/extract.py` currently returns page text and can render a cover. It can be extended to return metadata and table of contents from PyMuPDF.
- `go.mod` currently includes web and database dependencies (`echo`, `modernc.org/sqlite`, `uuid`, bcrypt via `x/crypto`). Implementation should remove unused dependencies after the web/database code is gone.
- `books/` and `results/` already exist but are empty.

## API Design
- Compatibility classification: breaking change by design.
- Current HTTP API and UI routes will be removed:
  - `/login`, `/logout`, `/library`
  - `/books/upload`
  - `/books`, `/books/:id/status`, `/books/:id`, `/books/:id/read`
  - `/books/:id/pages/:num`, `/books/:id/progress`, `/books/:id/cover`, `/books/:id/retry`, `/books/:id/shelf`
  - `/shelves`
  - `/health`
- No replacement HTTP API is planned for the MVP revamp.
- New user interface is command execution:
  - `go run ./cmd/aksara`
  - or `docker compose run --rm app`
- Proposed flags/env:
  - `BOOKS_DIR`, default `./books`
  - `RESULTS_DIR`, default `./results`
  - `DEEPSEEK_API_KEY`, required
  - `DEEPSEEK_MODEL`, default `deepseek-chat`
  - `PYTHON_BIN`, default `python3`
  - `PARSER_SCRIPT`, default `parser/extract.py`
  - `OVERWRITE`, default `false`
  - `TRANSLATION_CONCURRENCY`, default `1`
  - `TRANSLATION_TIMEOUT`, default `120s`
  - `TRANSLATION_RETRIES`, default `2`
- Exit behavior:
  - Exit `0` when all discovered books are already done or translated successfully.
  - Exit non-zero when any book fails.
  - Continue to the next book after a per-book failure unless a fatal config error occurs.

## Data And Migration Plan
- Database engine assumption: current app uses local SQLite through `modernc.org/sqlite`.
- Classification: application contract change plus data-retirement plan, not a production schema migration.
- Do not mutate, migrate, or delete existing `data/ai-reader.db`.
- Do not delete existing uploaded PDFs in `storage/pdfs`.
- The new translator ignores the old SQLite database and old storage layout by default.
- If old uploaded PDFs should be reprocessed, the user can manually move/copy them from `storage/pdfs/` into `books/` before running the translator.
- Rollback is simple at the source level: keep the old commit/branch available. Runtime rollback only works if old `data/` and `storage/` are preserved.

## Implementation Plan
1. Create a new command entrypoint.
   - Prefer `cmd/aksara/main.go`.
   - Load config from `.env`, env vars, and simple flags where practical.
   - Handle `SIGINT`/`SIGTERM` with `context.Context` cancellation.
   - Print a concise run summary: discovered, skipped, translated, failed.

2. Replace web-oriented config with translator config.
   - Remove required auth/session/server settings.
   - Add `BooksDir`, `ResultsDir`, `Overwrite`, `TranslationConcurrency`, `TranslationTimeout`, and `TranslationRetries`.
   - Resolve relative `PARSER_SCRIPT`, `BOOKS_DIR`, and `RESULTS_DIR` from the working directory.
   - Validate that `DEEPSEEK_API_KEY` exists, `books/` exists or can be created, and `results/` can be created.

3. Add filesystem book discovery.
   - Create a small package or service that scans `books/` for `.pdf`.
   - Ignore hidden files and non-PDF files.
   - Use deterministic ordering by filename.
   - Derive a safe output slug from the PDF filename.
   - Skip books whose output EPUB already exists unless overwrite is enabled.

4. Extend PDF extraction.
   - Keep Python as a subprocess.
   - Extend `parser/extract.py` to return:
     - document metadata title/author when present
     - page count
     - page text
     - table of contents from `doc.get_toc()` when available
   - Keep cover extraction as best-effort if EPUB cover support is included.
   - In Go, wrap subprocess errors with command context, exit code, and sanitized stderr. Do not log full extracted text.

5. Redesign translation output contract.
   - Update the DeepSeek system prompt away from web HTML.
   - Preferred contract: model returns JSON blocks, for example paragraphs, headings, lists, and code blocks.
   - Go validates the JSON response and escapes text while building XHTML for EPUB.
   - If JSON parsing fails, retry once with a repair prompt or fail the current chunk with a clear error.
   - Preserve code blocks exactly and keep software engineering terms in English.

6. Implement chunk planning.
   - Use table of contents when available to group pages into chapters.
   - If no table of contents exists, group consecutive pages into chunks bounded by character count.
   - Keep chunk sizes conservative to reduce model truncation and improve retryability.
   - Track source page ranges in logs and checkpoint files.

7. Add checkpoint/cache files.
   - Replace SQLite caching with filesystem checkpoints, for example `results/.cache/<slug>/chunk-0001.json`.
   - Store extracted metadata and translated block output per chunk.
   - Resume from cached chunks on retry to avoid retranslating expensive completed work.
   - Write cache files atomically: write temp file, fsync when practical, rename.
   - Do not treat cache files as the final user-facing result.

8. Build EPUB files with the Go standard library.
   - Avoid a new dependency unless implementation proves the standard-library path is too costly.
   - Use `archive/zip` to write:
     - `mimetype` first and uncompressed
     - `META-INF/container.xml`
     - `OEBPS/content.opf`
     - `OEBPS/nav.xhtml`
     - one XHTML content file per chapter/chunk
     - optional cover image if extraction succeeds
   - Generate EPUB 3-compatible metadata with title, language `id`, and identifier.
   - Use XML/XHTML escaping for all model-provided text.
   - Write to `results/<slug>.epub.tmp` and rename to `results/<slug>.epub` only after successful ZIP creation.

9. Remove old web/database application surface.
   - Remove Echo server wiring and route handlers.
   - Remove auth middleware.
   - Remove SQLite migration and old DB-dependent services.
   - Remove `web/templates`.
   - Remove screenshot-heavy web app README sections or rewrite README for the new CLI/job workflow.
   - Remove unused Go dependencies after source changes are complete.

10. Update Docker and local workflow.
    - Dockerfile builds the new command binary.
    - Runtime still includes Python and installs `parser/requirements.txt`.
    - Compose should mount:
      - `./books:/app/books`
      - `./results:/app/results`
    - Remove port mapping and server restart policy.
    - Prefer `docker compose run --rm app` for one-shot execution.
    - Update Makefile targets, for example `translate`, `docker-translate`, and optional `clean-cache`.

11. Update documentation.
    - Rewrite README to describe the translator-only workflow.
    - Document folder layout:
      - `books/*.pdf` input
      - `results/*.epub` output
      - `results/.cache/` resumable translation cache
    - Document env vars and examples.
    - Explain that no standalone HTML UI remains, while EPUB internally contains XHTML by specification.

## Error Handling Plan
- Validation errors:
  - Missing `DEEPSEEK_API_KEY`: fatal config error, no retry.
  - Missing or unreadable `books/`: fatal or create directory with a clear "no PDFs found" result, based on implementation choice.
  - Invalid PDF or no extractable text: per-book failure, continue next book.
- External service errors:
  - DeepSeek network timeout, 429, 5xx: retry with bounded exponential backoff and jitter when the chunk has not been committed to cache yet.
  - DeepSeek 400/401/403: permanent failure, fail fast because retries will not help.
  - Empty or malformed model response: retry once with repair prompt, then fail the chunk.
- Cancellation:
  - CLI context cancellation should stop new work and allow the current subprocess/HTTP request to terminate through context where possible.
  - Keep completed checkpoints so rerun resumes.
- File safety:
  - Never replace an existing EPUB unless overwrite is explicitly enabled.
  - Always create final EPUB through a temp file plus rename.
  - Do not delete source PDFs.
- Privacy:
  - Do not log raw book text, prompts, model responses, API keys, or full request/response bodies.
  - Logs can include book filename, chunk number, page range, status, duration, and error class.

## Observability Plan
- Use simple structured-ish log lines through the standard library or existing lightweight logging.
- Log run start/end:
  - input dir
  - output dir
  - model name
  - count of discovered/skipped/succeeded/failed books
- Log per book:
  - start, skip, extraction complete, chunk progress, EPUB written, failure
  - duration and page/chunk counts
- Log DeepSeek failures by class without response payloads.
- Metrics/tracing are not necessary for MVP because this becomes a local one-shot job, not a long-running service.
- Avoid per-paragraph logs and any high-volume raw text logging.

## Test Plan
- Unit tests:
  - config loading and validation
  - PDF discovery and slug generation
  - output skip/overwrite decisions
  - chunk planning from pages and optional table of contents
  - translator response validation for valid JSON, invalid JSON, empty response, and API error bodies
  - EPUB builder creates required files and escapes content
- Integration-style tests:
  - pipeline with fake extractor and fake translator writes a valid `.epub`
  - checkpoint resume skips already translated chunks
  - atomic output leaves no final EPUB when EPUB build fails
  - cancellation stops before starting additional books/chunks
- Python validation:
  - `python3 -m py_compile parser/extract.py`
  - optional local smoke test with a tiny text PDF fixture if available
- Go validation commands:
  - `go fmt ./...`
  - `go test ./...`
  - `go vet ./...`
- Manual validation:
  - Place one real PDF in `books/`.
  - Run `go run ./cmd/aksara`.
  - Confirm `results/<book>.epub` opens in Apple Books, Calibre, or another EPUB reader.
  - Interrupt midway, rerun, and confirm cached chunks are reused.

## Risks & Mitigations
- Risk: EPUB still contains XHTML internally, which may look like "HTML was not removed".
  - Mitigation: remove web/templates and standalone HTML outputs; document that internal XHTML is required by EPUB.
- Risk: Page-by-page translation can break paragraphs and chapter flow.
  - Mitigation: group pages into TOC-based or bounded chunks before translation.
- Risk: LLM returns malformed structured output.
  - Mitigation: validate response, retry repair once, and keep the failed chunk isolated.
- Risk: API cost from reruns.
  - Mitigation: checkpoint translated chunks and skip existing EPUBs by default.
- Risk: Large books may take hours and hit rate limits.
  - Mitigation: default concurrency `1`, bounded retries, progress logs, resumable cache.
- Risk: Removing web/API is breaking for current usage.
  - Mitigation: document the intentional contract change and keep old runtime data untouched.
- Risk: Old SQLite data or uploaded PDFs could be lost during cleanup.
  - Mitigation: implementation must not delete `data/` or `storage/`; cleanup should be separate and explicit.
- Risk: Generated EPUB structure may be invalid.
  - Mitigation: add EPUB builder tests that unzip and verify required files; manually open output in a reader.
- Risk: Logging sensitive content.
  - Mitigation: log metadata and error classes only, never raw text, prompts, responses, or secrets.

## Open Questions
- Should `books/` scanning be direct-only for MVP, or recursive?
- Should existing EPUB outputs always skip by default, or should overwrite be easier from the Makefile?
- Should translated checkpoint cache be kept forever to avoid future API cost, or removed after successful EPUB generation for a cleaner `results/` folder?
- Should EPUB metadata use only PDF metadata, or allow a sidecar file such as `books/<name>.json` for title/author/language/cover overrides?
- Is cover image extraction required for the first revamp, or can it be best-effort?

## Implementation Handoff
- When implementing this plan, automatically use `tdd-workflow` for backend Go changes: write tests first, prove RED, implement minimal code, prove GREEN, then refactor.
- When implementing this plan, automatically follow `coding-standard` for any Go code touched.
- For Go changes, automatically apply `go-patterns` for idiomatic backend design and implementation.
- For Go changes, use `go-lint-workflow` for scoped or baseline-aware lint.
- If Go validation fails during implementation, use `go-build-resolver` for minimal scoped fixes.
- For validation failures, external service failures, timeouts, retries, cancellation, workers, jobs, and observable failure behavior, use `error-handling` for error classification, propagation, boundary mapping, retry safety, logging privacy, and error-path tests.
- For the intentional API removal, use `api-contract-review` to keep compatibility notes and docs honest.
- For the local job pipeline, DeepSeek calls, retries, and progress logs, use `observability-check` for logging, error context, and sensitive-data safety.
- For retiring the SQLite-backed app state, use `database-migrations` as a safety lens: do not mutate or delete old data automatically, and document rollback/cleanup explicitly.
- Keep implementation scoped to this approved translator-only plan unless the user approves changes outside scope.
- Run focused format, lint/type-check, and test commands listed in `Test Plan` before finishing when feasible.
