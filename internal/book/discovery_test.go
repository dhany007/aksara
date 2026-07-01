package book

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverFindsPDFAndEPUBInputs(t *testing.T) {
	booksDir := t.TempDir()
	resultsDir := t.TempDir()

	touch(t, filepath.Join(booksDir, "zeta.epub"))
	touch(t, filepath.Join(booksDir, "Alpha Novel.PDF"))
	touch(t, filepath.Join(booksDir, "notes.txt"))
	touch(t, filepath.Join(booksDir, ".hidden.pdf"))
	touch(t, filepath.Join(resultsDir, "alpha-novel-indonesia.epub"))

	books, err := Discover(booksDir, resultsDir, false)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	if len(books) != 2 {
		t.Fatalf("len(books) = %d", len(books))
	}
	if books[0].Slug != "alpha-novel" || books[0].Format != FormatPDF || books[0].Status != StatusDone {
		t.Fatalf("unexpected first book: %#v", books[0])
	}
	if books[0].OutputPath != filepath.Join(resultsDir, "alpha-novel-indonesia.epub") {
		t.Fatalf("OutputPath = %q", books[0].OutputPath)
	}
	if books[1].Slug != "zeta" || books[1].Format != FormatEPUB || books[1].Status != StatusPending {
		t.Fatalf("unexpected second book: %#v", books[1])
	}
}

func TestDiscoverOverwriteMarksExistingOutputPending(t *testing.T) {
	booksDir := t.TempDir()
	resultsDir := t.TempDir()

	touch(t, filepath.Join(booksDir, "Novel.pdf"))
	touch(t, filepath.Join(resultsDir, "novel-indonesia.epub"))

	books, err := Discover(booksDir, resultsDir, true)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	if len(books) != 1 {
		t.Fatalf("len(books) = %d", len(books))
	}
	if books[0].Status != StatusPending {
		t.Fatalf("Status = %s", books[0].Status)
	}
}

func TestSlugifyKeepsReadableASCIIName(t *testing.T) {
	if got := Slugify("The Hero's Journey (Book 1).pdf"); got != "the-hero-s-journey-book-1" {
		t.Fatalf("Slugify returned %q", got)
	}
}

func TestBookPreviewUsesSeparateOutputAndCacheSlug(t *testing.T) {
	resultsDir := t.TempDir()
	input := Book{
		Path:       filepath.Join(t.TempDir(), "Novel.pdf"),
		Title:      "Novel",
		Slug:       "novel",
		Format:     FormatPDF,
		OutputPath: filepath.Join(resultsDir, "novel-indonesia.epub"),
		Status:     StatusDone,
	}

	preview, err := input.Preview(3, false)
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if preview.Slug != "novel-preview-3p" {
		t.Fatalf("Slug = %q", preview.Slug)
	}
	if preview.OutputPath != filepath.Join(resultsDir, "novel-indonesia-preview-3p.epub") {
		t.Fatalf("OutputPath = %q", preview.OutputPath)
	}
	if preview.Status != StatusPending {
		t.Fatalf("Status = %s", preview.Status)
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
