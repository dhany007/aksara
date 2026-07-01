# TDD Evidence - EPUB Translator Revamp

## Source Plan
- `tech-plan-epub-translator-revamp.md`
- Follow-up user context:
  - Run from terminal, not hosted.
  - Use default phone/tablet EPUB readers.
  - Support resume from chunk cache.
  - Support `.pdf` and DRM-free `.epub` inputs.
  - Prioritize novel comprehension and story flow.

## Testable Guarantees

| # | Guarantee | Test target | Test type | RED evidence | GREEN evidence |
|---|-----------|-------------|-----------|--------------|----------------|
| 1 | Translator config loads new env defaults and rejects missing API key. | `internal/config/config_test.go` | unit | `go test ./internal/...` failed with `undefined: FromEnv`. | `go test ./...` passed. |
| 2 | Discovery reads `.pdf` and `.epub`, ignores hidden/non-book files, and marks existing EPUB output as done unless overwrite is enabled. | `internal/book/discovery_test.go` | unit | `go test ./internal/...` failed with `undefined: Discover`, `FormatPDF`, `StatusDone`. | `go test ./...` passed. |
| 3 | Chunk planning preserves front matter and plain text block parsing preserves code fences. | `internal/content/chunk_test.go` | unit | `go test ./internal/...` failed with `undefined: SourceDocument`, `PlanChunks`, `BlocksFromPlainText`. | `go test ./...` passed. |
| 4 | Chunk cache saves translated chunks as JSON and reports missing chunks with `ErrNotFound`. | `internal/cache/store_test.go` | unit | `go test ./internal/...` failed because package had no implementation files. | `go test ./...` passed. |
| 5 | EPUB builder writes a valid EPUB zip skeleton, with uncompressed `mimetype`, OPF/nav/chapter files, and escaped content. | `internal/epub/builder_test.go` | unit | `go test ./internal/...` failed because package had no implementation files. | `go test ./...` passed. |
| 6 | DeepSeek client parses JSON block output and classifies auth failures as permanent. | `internal/service/translator_test.go` | unit | `go test ./internal/...` failed because `TranslatorConfig` and `TranslateBlocks` did not exist. | `go test ./...` passed. |
| 7 | Pipeline resumes from cached chunks and writes final EPUB without retranslating completed chunks. | `internal/pipeline/pipeline_test.go` | integration-style unit | `go test ./internal/...` failed because package had no implementation files. | `go test ./...` passed. |

## Validation Commands

- `go test ./internal/...`: initial RED, failed on missing new API/package implementations.
- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `python3 -m py_compile parser/extract.py`: PASS.
- `DEEPSEEK_API_KEY=sk-test BOOKS_DIR=/private/tmp/aksara-empty-books RESULTS_DIR=/private/tmp/aksara-empty-results go run ./cmd/aksara`: PASS, found 0 books and made no API call.

## Coverage / Gaps

- Covered:
  - config parsing
  - filesystem discovery
  - chunk planning
  - cache save/load
  - EPUB package generation
  - translator JSON response parsing and permanent auth error classification
  - pipeline resume behavior
- Gaps:
  - No real DeepSeek API call was run.
  - No real PDF or real EPUB fixture smoke test was run.
  - Docker image build was not run because it would install Python dependencies.
  - EPUB output was validated structurally by tests, but not opened in a reader app.

## Notes

- Old SQLite/web runtime data is ignored, not deleted.
- The hosted HTML reader was removed. EPUB internals still use XHTML because EPUB requires it.
