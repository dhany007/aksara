package pipeline

import (
	"context"
	"errors"
	"fmt"
	"net"
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
	MaxPages               int
	TranslationRetries     int
	TranslationConcurrency int
	Extractor              Extractor
	Translator             Translator
	Builder                Builder
	Progress               ProgressReporter
}

type Runner struct {
	resultsDir             string
	maxChunkChars          int
	maxPages               int
	translationRetries     int
	translationConcurrency int
	store                  *cache.Store
	extractor              Extractor
	translator             Translator
	builder                Builder
	progress               ProgressReporter
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
		maxPages:               opts.MaxPages,
		translationRetries:     opts.TranslationRetries,
		translationConcurrency: opts.TranslationConcurrency,
		store:                  cache.NewStore(opts.ResultsDir),
		extractor:              opts.Extractor,
		translator:             opts.Translator,
		builder:                opts.Builder,
		progress:               opts.Progress,
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
	source = content.LimitPages(source, r.maxPages)
	chunks := content.PlanChunks(source, r.maxChunkChars)
	if len(chunks) == 0 {
		err := fmt.Errorf("no translatable text found")
		result.Status = StatusFailed
		result.Error = err
		result.Duration = time.Since(start)
		return result, err
	}
	r.report(ProgressEvent{
		Stage:   ProgressStageChunks,
		Message: fmt.Sprintf("planned %d chunk(s)", len(chunks)),
		Current: 0,
		Total:   len(chunks),
	})

	translated := content.TranslatedDocument{
		Title:    firstNonEmpty(source.Title, input.Title),
		Author:   source.Author,
		Language: "id",
		Cover:    source.Cover,
		Chapters: make([]content.TranslatedChunk, len(chunks)),
	}
	var missing []missingChunk
	cached := 0
	for _, chunk := range chunks {
		done, err := r.store.LoadChunk(input.Slug, chunk.Number)
		if err == nil {
			translated.Chapters[chunk.Number-1] = done
			cached++
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
	r.report(ProgressEvent{
		Stage:   ProgressStageCache,
		Message: fmt.Sprintf("cache ready: %d/%d chunk(s), %d missing", cached, len(chunks), len(missing)),
		Current: cached,
		Total:   len(chunks),
	})
	if len(missing) > 0 {
		doneChunks, err := r.translateMissing(ctx, input.Slug, missing, cached, len(chunks))
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

	r.report(ProgressEvent{
		Stage:   ProgressStageBuild,
		Message: fmt.Sprintf("building EPUB: %s", input.OutputPath),
		Current: len(chunks),
		Total:   len(chunks),
	})
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
	r.report(ProgressEvent{Stage: ProgressStageSource, Message: "checking source cache"})
	source, err := r.store.LoadSource(input.Slug)
	if err == nil {
		r.report(ProgressEvent{
			Stage:   ProgressStageSource,
			Message: fmt.Sprintf("source cache loaded: %d section(s)", len(source.Sections)),
		})
		return source, nil
	}
	if !errors.Is(err, cache.ErrNotFound) {
		return content.SourceDocument{}, err
	}
	r.report(ProgressEvent{
		Stage:   ProgressStageSource,
		Message: fmt.Sprintf("extracting source text from %s; this can take a while", input.Format),
	})
	source, err = r.extractor.Extract(ctx, input, r.store.BookDir(input.Slug))
	if err != nil {
		return content.SourceDocument{}, fmt.Errorf("extract %s: %w", input.Path, err)
	}
	if err := r.store.SaveSource(input.Slug, source); err != nil {
		return content.SourceDocument{}, err
	}
	r.report(ProgressEvent{
		Stage:   ProgressStageSource,
		Message: fmt.Sprintf("source extracted: %d section(s)", len(source.Sections)),
	})
	return source, nil
}

type missingChunk struct {
	chunk content.Chunk
}

func (r *Runner) translateMissing(ctx context.Context, slug string, missing []missingChunk, completed int, total int) ([]content.TranslatedChunk, error) {
	if r.translationConcurrency <= 1 || len(missing) == 1 {
		done := make([]content.TranslatedChunk, 0, len(missing))
		for _, item := range missing {
			r.report(ProgressEvent{
				Stage:   ProgressStageTranslate,
				Message: fmt.Sprintf("translating chunk %d/%d", item.chunk.Number, total),
				Current: completed,
				Total:   total,
			})
			chunk, err := r.translateAndCache(ctx, slug, item.chunk)
			if err != nil {
				return nil, err
			}
			done = append(done, chunk)
			completed++
			r.report(ProgressEvent{
				Stage:   ProgressStageTranslate,
				Message: fmt.Sprintf("translated chunk %d/%d", chunk.Number, total),
				Current: completed,
				Total:   total,
			})
		}
		return done, nil
	}

	r.report(ProgressEvent{
		Stage:   ProgressStageTranslate,
		Message: fmt.Sprintf("translating %d missing chunk(s) with concurrency %d", len(missing), r.translationConcurrency),
		Current: completed,
		Total:   total,
	})

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
		completed++
		r.report(ProgressEvent{
			Stage:   ProgressStageTranslate,
			Message: fmt.Sprintf("translated chunk %d/%d", result.chunk.Number, total),
			Current: completed,
			Total:   total,
		})
	}
	if err := ctx.Err(); err != nil && len(done) != len(missing) {
		return nil, err
	}
	return done, nil
}

func (r *Runner) report(event ProgressEvent) {
	if r.progress != nil {
		r.progress(event)
	}
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
		r.report(ProgressEvent{
			Stage:   ProgressStageTranslate,
			Message: fmt.Sprintf("retrying chunk %d after transient error (%d/%d)", chunk.Number, attempt+1, r.translationRetries),
		})
	}
	return nil, fmt.Errorf("translate chunk %d: %w", chunk.Number, lastErr)
}

func isTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "transient") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "deadline exceeded")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
