package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"aksara/internal/book"
	"aksara/internal/cache"
	"aksara/internal/content"
)

type Status string

const (
	StatusTranslated Status = "translated"
	StatusSkipped    Status = "skipped"
	StatusFailed     Status = "failed"
)

type Result struct {
	Book     book.Book
	Status   Status
	Chunks   int
	Duration time.Duration
	Error    error
}

type Extractor interface {
	Extract(ctx context.Context, input book.Book, cacheDir string) (content.SourceDocument, error)
}

type Translator interface {
	TranslateBlocks(ctx context.Context, text string) ([]content.Block, error)
}

type Builder interface {
	Build(path string, doc content.TranslatedDocument) error
}

type RunnerOptions struct {
	ResultsDir             string
	MaxChunkChars          int
	TranslationRetries     int
	TranslationConcurrency int
	Extractor              Extractor
	Translator             Translator
	Builder                Builder
}

type Runner struct {
	resultsDir             string
	maxChunkChars          int
	translationRetries     int
	translationConcurrency int
	store                  *cache.Store
	extractor              Extractor
	translator             Translator
	builder                Builder
}

func NewRunner(opts RunnerOptions) *Runner {
	if opts.MaxChunkChars <= 0 {
		opts.MaxChunkChars = 7000
	}
	if opts.TranslationConcurrency <= 0 {
		opts.TranslationConcurrency = 1
	}
	return &Runner{
		resultsDir:             opts.ResultsDir,
		maxChunkChars:          opts.MaxChunkChars,
		translationRetries:     opts.TranslationRetries,
		translationConcurrency: opts.TranslationConcurrency,
		store:                  cache.NewStore(opts.ResultsDir),
		extractor:              opts.Extractor,
		translator:             opts.Translator,
		builder:                opts.Builder,
	}
}

func (r *Runner) Cache() *cache.Store {
	return r.store
}

func (r *Runner) Process(ctx context.Context, input book.Book) (Result, error) {
	start := time.Now()
	result := Result{Book: input}
	if input.Status == book.StatusDone {
		result.Status = StatusSkipped
		result.Duration = time.Since(start)
		return result, nil
	}
	if _, err := os.Stat(input.OutputPath); err == nil {
		result.Status = StatusSkipped
		result.Duration = time.Since(start)
		return result, nil
	} else if err != nil && !os.IsNotExist(err) {
		return result, fmt.Errorf("stat output epub: %w", err)
	}

	source, err := r.loadOrExtract(ctx, input)
	if err != nil {
		result.Status = StatusFailed
		result.Error = err
		result.Duration = time.Since(start)
		return result, err
	}
	chunks := content.PlanChunks(source, r.maxChunkChars)
	if len(chunks) == 0 {
		err := fmt.Errorf("no translatable text found")
		result.Status = StatusFailed
		result.Error = err
		result.Duration = time.Since(start)
		return result, err
	}

	translated := content.TranslatedDocument{
		Title:    firstNonEmpty(source.Title, input.Title),
		Author:   source.Author,
		Language: "id",
		Cover:    source.Cover,
		Chapters: make([]content.TranslatedChunk, len(chunks)),
	}
	var missing []missingChunk
	for _, chunk := range chunks {
		done, err := r.store.LoadChunk(input.Slug, chunk.Number)
		if err == nil {
			translated.Chapters[chunk.Number-1] = done
			continue
		}
		if !errors.Is(err, cache.ErrNotFound) {
			result.Status = StatusFailed
			result.Error = err
			result.Duration = time.Since(start)
			return result, err
		}

		missing = append(missing, missingChunk{chunk: chunk})
	}
	if len(missing) > 0 {
		doneChunks, err := r.translateMissing(ctx, input.Slug, missing)
		if err != nil {
			result.Status = StatusFailed
			result.Error = err
			result.Duration = time.Since(start)
			return result, err
		}
		for _, done := range doneChunks {
			translated.Chapters[done.Number-1] = done
		}
	}

	if err := r.builder.Build(input.OutputPath, translated); err != nil {
		result.Status = StatusFailed
		result.Error = err
		result.Duration = time.Since(start)
		return result, err
	}
	result.Status = StatusTranslated
	result.Chunks = len(chunks)
	result.Duration = time.Since(start)
	return result, nil
}

func (r *Runner) loadOrExtract(ctx context.Context, input book.Book) (content.SourceDocument, error) {
	source, err := r.store.LoadSource(input.Slug)
	if err == nil {
		return source, nil
	}
	if !errors.Is(err, cache.ErrNotFound) {
		return content.SourceDocument{}, err
	}
	source, err = r.extractor.Extract(ctx, input, r.store.BookDir(input.Slug))
	if err != nil {
		return content.SourceDocument{}, fmt.Errorf("extract %s: %w", input.Path, err)
	}
	if err := r.store.SaveSource(input.Slug, source); err != nil {
		return content.SourceDocument{}, err
	}
	return source, nil
}

type missingChunk struct {
	chunk content.Chunk
}

func (r *Runner) translateMissing(ctx context.Context, slug string, missing []missingChunk) ([]content.TranslatedChunk, error) {
	if r.translationConcurrency <= 1 || len(missing) == 1 {
		done := make([]content.TranslatedChunk, 0, len(missing))
		for _, item := range missing {
			chunk, err := r.translateAndCache(ctx, slug, item.chunk)
			if err != nil {
				return nil, err
			}
			done = append(done, chunk)
		}
		return done, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan content.Chunk)
	results := make(chan translateResult, len(missing))
	var wg sync.WaitGroup
	for i := 0; i < r.translationConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for chunk := range jobs {
				done, err := r.translateAndCache(ctx, slug, chunk)
				if err != nil {
					cancel()
				}
				results <- translateResult{chunk: done, err: err}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, item := range missing {
			select {
			case <-ctx.Done():
				return
			case jobs <- item.chunk:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	done := make([]content.TranslatedChunk, 0, len(missing))
	for result := range results {
		if result.err != nil {
			return nil, result.err
		}
		done = append(done, result.chunk)
	}
	if err := ctx.Err(); err != nil && len(done) != len(missing) {
		return nil, err
	}
	return done, nil
}

type translateResult struct {
	chunk content.TranslatedChunk
	err   error
}

func (r *Runner) translateAndCache(ctx context.Context, slug string, chunk content.Chunk) (content.TranslatedChunk, error) {
	blocks, err := r.translateWithRetry(ctx, chunk)
	if err != nil {
		return content.TranslatedChunk{}, err
	}
	done := content.TranslatedChunk{
		Number: chunk.Number,
		Title:  firstNonEmpty(chunk.Title, fmt.Sprintf("Bagian %d", chunk.Number)),
		Start:  chunk.Start,
		End:    chunk.End,
		Blocks: blocks,
	}
	if err := r.store.SaveChunk(slug, done); err != nil {
		return content.TranslatedChunk{}, err
	}
	return done, nil
}

func (r *Runner) translateWithRetry(ctx context.Context, chunk content.Chunk) ([]content.Block, error) {
	var lastErr error
	for attempt := 0; attempt <= r.translationRetries; attempt++ {
		blocks, err := r.translator.TranslateBlocks(ctx, chunk.Text)
		if err == nil {
			if len(blocks) == 0 {
				blocks = content.BlocksFromPlainText(chunk.Text)
			}
			return blocks, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !isTransient(err) || attempt == r.translationRetries {
			break
		}
	}
	return nil, fmt.Errorf("translate chunk %d: %w", chunk.Number, lastErr)
}

func isTransient(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "transient") || strings.Contains(err.Error(), "timeout"))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
