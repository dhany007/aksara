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
	touch(t, filepath.Join(resultsDir, "alpha-novel.epub"))

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
	if books[1].Slug != "zeta" || books[1].Format != FormatEPUB || books[1].Status != StatusPending {
		t.Fatalf("unexpected second book: %#v", books[1])
	}
}

func TestDiscoverOverwriteMarksExistingOutputPending(t *testing.T) {
	booksDir := t.TempDir()
	resultsDir := t.TempDir()

	touch(t, filepath.Join(booksDir, "Novel.pdf"))
	touch(t, filepath.Join(resultsDir, "novel.epub"))

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

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
