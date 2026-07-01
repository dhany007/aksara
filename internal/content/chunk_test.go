package content

import "testing"

func TestPlanChunksSplitsSectionsWithoutDroppingFrontMatter(t *testing.T) {
	doc := SourceDocument{
		Title: "Novel",
		Sections: []SourceSection{
			{Title: "Title Page", Start: 1, End: 1, Text: "A short title page."},
			{Title: "Preface", Start: 2, End: 4, Text: "Preface paragraph one.\n\nPreface paragraph two."},
			{Title: "Chapter 1", Start: 5, End: 8, Text: "This is a longer chapter that should be split into more than one translated chunk."},
		},
	}

	chunks := PlanChunks(doc, 48)
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d: %#v", len(chunks), chunks)
	}
	if chunks[0].Title != "Title Page" || chunks[0].Start != 1 || chunks[0].End != 1 {
		t.Fatalf("front matter was not preserved first: %#v", chunks[0])
	}
	if chunks[0].Text == "" {
		t.Fatal("first chunk text is empty")
	}
	for i, chunk := range chunks {
		if chunk.Number != i+1 {
			t.Fatalf("chunk number = %d, want %d", chunk.Number, i+1)
		}
		if len(chunk.Text) > 80 {
			t.Fatalf("chunk %d is unexpectedly large: %d", chunk.Number, len(chunk.Text))
		}
	}
}

func TestBlocksFromPlainTextPreservesParagraphsAndCodeFences(t *testing.T) {
	blocks := BlocksFromPlainText("Hello world.\n\n```go\nfmt.Println(\"hi\")\n```\n\nAfter code.")
	if len(blocks) != 3 {
		t.Fatalf("len(blocks) = %d: %#v", len(blocks), blocks)
	}
	if blocks[0].Type != BlockParagraph || blocks[0].Text != "Hello world." {
		t.Fatalf("unexpected paragraph block: %#v", blocks[0])
	}
	if blocks[1].Type != BlockCode || blocks[1].Text != "fmt.Println(\"hi\")" {
		t.Fatalf("unexpected code block: %#v", blocks[1])
	}
	if blocks[2].Type != BlockParagraph || blocks[2].Text != "After code." {
		t.Fatalf("unexpected trailing block: %#v", blocks[2])
	}
}
