# Stage 1: build Go CLI
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o aksara ./cmd/aksara

# Stage 2: runtime with Python + PyMuPDF for PDF extraction
FROM python:3.11-slim
WORKDIR /app

COPY parser/requirements.txt ./parser/requirements.txt
RUN pip install --no-cache-dir -r parser/requirements.txt

COPY --from=builder /app/aksara .
COPY parser/ ./parser/

RUN mkdir -p /app/books /app/results

ENV BOOKS_DIR=/app/books \
    RESULTS_DIR=/app/results \
    PYTHON_BIN=python3 \
    PARSER_SCRIPT=/app/parser/extract.py \
    TRANSLATION_CONCURRENCY=1 \
    TRANSLATION_RETRIES=2 \
    TRANSLATION_TIMEOUT=120s \
    OVERWRITE=false

CMD ["./aksara"]
