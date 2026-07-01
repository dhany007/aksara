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
		OutputPath: filepath.Join(resultsDir, "novel-indonesia.epub"),
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
	if _, err := os.Stat(filepath.Join(resultsDir, "novel-indonesia.epub")); err != nil {
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
	doc    content.TranslatedDocument
}

func (f *fakeBuilder) Build(path string, doc content.TranslatedDocument) error {
	f.called = true
	f.doc = doc
	return os.WriteFile(path, []byte("epub"), 0644)
}

func TestRunnerLimitsPreviewToMaxPages(t *testing.T) {
	resultsDir := t.TempDir()
	source := content.SourceDocument{
		Title: "Novel",
		Sections: []content.SourceSection{
			{Title: "Page 1", Start: 1, End: 1, Text: "Page one."},
			{Title: "Page 2", Start: 2, End: 2, Text: "Page two."},
			{Title: "Page 3", Start: 3, End: 3, Text: "Page three."},
			{Title: "Page 4", Start: 4, End: 4, Text: "Page four."},
		},
	}
	translator := &fakeTranslator{
		blocksByText: map[string][]content.Block{
			"Page one.":   {{Type: content.BlockParagraph, Text: "Halaman satu."}},
			"Page two.":   {{Type: content.BlockParagraph, Text: "Halaman dua."}},
			"Page three.": {{Type: content.BlockParagraph, Text: "Halaman tiga."}},
		},
	}
	builder := &fakeBuilder{}
	runner := NewRunner(RunnerOptions{
		ResultsDir:    resultsDir,
		MaxChunkChars: 32,
		MaxPages:      3,
		Extractor:     &fakeExtractor{source: source},
		Translator:    translator,
		Builder:       builder,
	})

	_, err := runner.Process(context.Background(), book.Book{
		Path:       filepath.Join(t.TempDir(), "Novel.pdf"),
		Slug:       "novel-preview-3p",
		Title:      "Novel",
		Format:     book.FormatPDF,
		OutputPath: filepath.Join(resultsDir, "novel-indonesia-preview-3p.epub"),
		Status:     book.StatusPending,
	})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if translator.calls != 3 {
		t.Fatalf("translator calls = %d", translator.calls)
	}
	if len(builder.doc.Chapters) != 3 {
		t.Fatalf("chapters = %d", len(builder.doc.Chapters))
	}
}
