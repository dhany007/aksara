# Aksara

Aksara translates English books into natural Indonesian and outputs EPUB files you can read in your phone or tablet's default book reader.

It is no longer a hosted e-reader. There is no upload UI, login, SQLite database, or web reader. Put books in `books/`, run the translator, and take the translated `.epub` from `results/`.

## Features

- Read source books from `books/`
- Supports `.pdf` and DRM-free `.epub` inputs
- Translate novels into natural Indonesian with DeepSeek API
- Preserve story flow, dialogue, character voice, names, places, and invented terms
- Generate EPUB output for Apple Books, Google Play Books, Moon+ Reader, Kobo, and similar readers
- Resume interrupted translation from JSON chunk cache
- Skip already finished books by default
- Best-effort PDF first-page cover extraction
- No hosting required

## Folder Layout

```text
aksara/
├── books/                 # input: .pdf or .epub
├── results/               # output: translated .epub
│   └── .cache/            # resumable per-book chunks
├── cmd/aksara/            # CLI entrypoint
├── internal/              # translator pipeline
├── parser/extract.py      # PDF extraction via PyMuPDF
├── .env
├── Dockerfile
└── docker-compose.yml
```

Example:

```text
books/
  novel-a.pdf
  novel-b.epub

results/
  novel-a-indonesia.epub
  novel-b-indonesia.epub
  .cache/
    novel-a/
      extract.json
      chunk-0001.json
      chunk-0002.json
```

## Setup

### Prerequisites

- DeepSeek API key
- For local runs: Go 1.25+, Python 3.10+, PyMuPDF
- Or Docker + Docker Compose

### Configure

```bash
cp .env.example .env
```

Edit `.env`:

```env
DEEPSEEK_API_KEY=sk-xxx
DEEPSEEK_MODEL=deepseek-v4-pro

BOOKS_DIR=./books
RESULTS_DIR=./results

TRANSLATION_CONCURRENCY=1
TRANSLATION_RETRIES=2
TRANSLATION_TIMEOUT=120s
MAX_CHUNK_CHARS=7000
MAX_PAGES=0
OVERWRITE=false

PYTHON_BIN=python3
PARSER_SCRIPT=parser/extract.py
```

## Usage

Put books into `books/`:

```text
books/my-novel.pdf
books/another-novel.epub
```

Run locally:

```bash
go run ./cmd/aksara
```

Or run with Docker:

```bash
docker compose run --rm app
```

To preview only the first 3 pages, set `MAX_PAGES=3` in `.env` or inline:

```bash
MAX_PAGES=3 go run ./cmd/aksara
```

Preview output uses a separate file and cache, for example:

```text
results/my-novel-indonesia-preview-3p.epub
results/.cache/my-novel-preview-3p/
```

Translated EPUB files appear in `results/`:

```text
results/my-novel-indonesia.epub
results/another-novel-indonesia.epub
```

## Resume Behavior

Aksara treats the final EPUB as the "done" marker:

- `results/book-indonesia.epub` exists: skip the book
- no final EPUB but cache exists: resume from missing chunks
- no final EPUB and no cache: start from scratch

If the terminal is closed midway, completed chunks remain in:

```text
results/.cache/book/chunk-0001.json
results/.cache/book/chunk-0002.json
```

Running Aksara again continues from the next missing chunk.

## Notes

- EPUB files internally contain XHTML by specification. Aksara removes the hosted HTML reader and does not write standalone HTML output.
- PDF image extraction beyond the cover is not the first priority. For image-heavy books, DRM-free EPUB input will generally preserve structure better.
- Existing old `data/` and `storage/` folders are not used by the new translator and are not deleted automatically.
