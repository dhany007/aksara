package cache

import (
	"errors"
	"testing"

	"aksara/internal/content"
)

func TestStoreSavesAndLoadsChunk(t *testing.T) {
	store := NewStore(t.TempDir())
	chunk := content.TranslatedChunk{
		Number: 1,
		Title:  "Chapter 1",
		Blocks: []content.Block{{Type: content.BlockParagraph, Text: "Halo dunia."}},
	}

	if err := store.SaveChunk("novel", chunk); err != nil {
		t.Fatalf("SaveChunk returned error: %v", err)
	}

	got, err := store.LoadChunk("novel", 1)
	if err != nil {
		t.Fatalf("LoadChunk returned error: %v", err)
	}
	if got.Title != chunk.Title || len(got.Blocks) != 1 || got.Blocks[0].Text != "Halo dunia." {
		t.Fatalf("loaded chunk = %#v", got)
	}
}

func TestStoreMissingChunkReturnsErrNotFound(t *testing.T) {
	store := NewStore(t.TempDir())
	_, err := store.LoadChunk("novel", 9)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("LoadChunk error = %v", err)
	}
}
