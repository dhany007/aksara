package book

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type Format string

const (
	FormatPDF  Format = "pdf"
	FormatEPUB Format = "epub"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusDone    Status = "done"
)

type Book struct {
	Path       string
	Title      string
	Slug       string
	Format     Format
	OutputPath string
	Status     Status
}

func Discover(booksDir, resultsDir string, overwrite bool) ([]Book, error) {
	entries, err := os.ReadDir(booksDir)
	if err != nil {
		return nil, fmt.Errorf("read books dir: %w", err)
	}

	var books []Book
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		format, ok := formatForName(entry.Name())
		if !ok {
			continue
		}
		slug := Slugify(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())))
		if slug == "" {
			slug = "book"
		}
		output := filepath.Join(resultsDir, slug+".epub")
		status := StatusPending
		if !overwrite {
			if _, err := os.Stat(output); err == nil {
				status = StatusDone
			} else if err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("stat output %s: %w", output, err)
			}
		}

		books = append(books, Book{
			Path:       filepath.Join(booksDir, entry.Name()),
			Title:      strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			Slug:       slug,
			Format:     format,
			OutputPath: output,
			Status:     status,
		})
	}

	sort.Slice(books, func(i, j int) bool {
		return strings.ToLower(books[i].Path) < strings.ToLower(books[j].Path)
	})
	return books, nil
}

func Slugify(name string) string {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func formatForName(name string) (Format, bool) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return FormatPDF, true
	case ".epub":
		return FormatEPUB, true
	default:
		return "", false
	}
}
