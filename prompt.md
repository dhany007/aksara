# Aksara Revamp Context

Aksara is now a local book translator, not a hosted e-reader.

Core idea:
- Put English `.pdf` or DRM-free `.epub` books in `books/`.
- Run Aksara from the terminal.
- Aksara translates the book into natural Indonesian.
- Aksara writes the translated book as `.epub` in `results/`.
- Read the output EPUB in a default phone/tablet book reader.

Primary use case:
- Help an Indonesian reader follow the story of English novels without translating sentence by sentence.
- Prioritize natural reading, story flow, dialogue, character voice, and consistency of names/places/invented terms.

Non-goals:
- No hosted web reader.
- No upload UI.
- No login/session auth.
- No SQLite database.
- No reading progress tracking.
- No shelves/library UI.
- No standalone HTML output.

Runtime behavior:
- `results/<slug>.epub` means the book is done and should be skipped by default.
- `results/.cache/<slug>/chunk-0001.json` and similar files store translated chunks for resume.
- If interrupted, rerunning continues from missing chunks.
- Existing old `data/` and `storage/` folders are ignored and should not be deleted automatically.

Default env:

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
