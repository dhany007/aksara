package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aksara/internal/book"
	"aksara/internal/content"
)

func TestRunnerResumesCachedChunksAndWritesEPUB(t *testing.T) {
	resultsDir := t.TempDir()
	source := content.SourceDocument{
		Title: "Novel",
		Sections: []content.SourceSection{
			{Title: "Chapter 1", Start: 1, End: 1, Text: "First chunk text."},
			{Title: "Chapter 2", Start: 2, End: 2, Text: "Second chunk text."},
		},
	}
	extractor := &fakeExtractor{source: source}
	translator := &fakeTranslator{
		blocksByText: map[string][]content.Block{
			"Second chunk text.": {{Type: content.BlockParagraph, Text: "Chunk kedua."}},
		},
	}
	builder := &fakeBuilder{}
	runner := NewRunner(RunnerOptions{
		ResultsDir:    resultsDir,
		MaxChunkChars: 32,
		Extractor:     extractor,
		Translator:    translator,
		Builder:       builder,
	})

	err := runner.Cache().SaveChunk("novel", content.TranslatedChunk{
		Number: 1,
		Title:  "Chapter 1",
		Blocks: []content.Block{{Type: content.BlockParagraph, Text: "Chunk pertama."}},
	})
	if err != nil {
		t.Fatalf("SaveChunk returned error: %v", err)
	}

	result, err := runner.Process(context.Background(), book.Book{
		Path:       filepath.Join(t.TempDir(), "Novel.pdf"),
		Slug:       "novel",
		Title:      "Novel",
		Format:     book.FormatPDF,
		OutputPath: filepath.Join(resultsDir, "novel.epub"),
		Status:     book.StatusPending,
	})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	if result.Status != StatusTranslated {
		t.Fatalf("Status = %s", result.Status)
	}
	if translator.calls != 1 {
		t.Fatalf("translator calls = %d", translator.calls)
	}
	if !builder.called {
		t.Fatal("builder was not called")
	}
	if _, err := os.Stat(filepath.Join(resultsDir, "novel.epub")); err != nil {
		t.Fatalf("output epub missing: %v", err)
	}
}

type fakeExtractor struct {
	source content.SourceDocument
}

func (f *fakeExtractor) Extract(ctx context.Context, input book.Book, cacheDir string) (content.SourceDocument, error) {
	return f.source, nil
}

type fakeTranslator struct {
	blocksByText map[string][]content.Block
	calls        int
}

func (f *fakeTranslator) TranslateBlocks(ctx context.Context, text string) ([]content.Block, error) {
	f.calls++
	return f.blocksByText[text], nil
}

type fakeBuilder struct {
	called bool
}

func (f *fakeBuilder) Build(path string, doc content.TranslatedDocument) error {
	f.called = true
	return os.WriteFile(path, []byte("epub"), 0644)
}
