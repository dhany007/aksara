package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"aksara/internal/content"
)

var ErrNotFound = errors.New("cache item not found")

type Store struct {
	root string
}

func NewStore(resultsDir string) *Store {
	return &Store{root: filepath.Join(resultsDir, ".cache")}
}

func (s *Store) BookDir(slug string) string {
	return filepath.Join(s.root, slug)
}

func (s *Store) SaveSource(slug string, source content.SourceDocument) error {
	return writeJSONAtomic(filepath.Join(s.BookDir(slug), "extract.json"), source)
}

func (s *Store) LoadSource(slug string) (content.SourceDocument, error) {
	var source content.SourceDocument
	if err := readJSON(filepath.Join(s.BookDir(slug), "extract.json"), &source); err != nil {
		return content.SourceDocument{}, err
	}
	return source, nil
}

func (s *Store) SaveChunk(slug string, chunk content.TranslatedChunk) error {
	return writeJSONAtomic(s.chunkPath(slug, chunk.Number), chunk)
}

func (s *Store) LoadChunk(slug string, number int) (content.TranslatedChunk, error) {
	var chunk content.TranslatedChunk
	if err := readJSON(s.chunkPath(slug, number), &chunk); err != nil {
		return content.TranslatedChunk{}, err
	}
	return chunk, nil
}

func (s *Store) chunkPath(slug string, number int) string {
	return filepath.Join(s.BookDir(slug), fmt.Sprintf("chunk-%04d.json", number))
}

func readJSON(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func writeJSONAtomic(path string, value interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache json: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp cache %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("commit cache %s: %w", path, err)
	}
	return nil
}
