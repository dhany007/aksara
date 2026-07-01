package epub

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"

	"aksara/internal/content"
)

func TestWriteCreatesEPUBPackage(t *testing.T) {
	var buf bytes.Buffer
	doc := Document{
		Title:    "Novel <One>",
		Author:   "Author",
		Language: "id",
		Chapters: []Chapter{
			{
				ID:    "chapter-1",
				Title: "Bab 1",
				Blocks: []content.Block{
					{Type: content.BlockParagraph, Text: "Halo <dunia> & semuanya."},
					{Type: content.BlockCode, Text: "if x < y {\n\tfmt.Println(x)\n}"},
				},
			},
		},
	}

	if err := Write(&buf, doc); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if len(zr.File) == 0 || zr.File[0].Name != "mimetype" || zr.File[0].Method != zip.Store {
		t.Fatalf("first file is not uncompressed mimetype: %#v", zr.File[0])
	}

	required := []string{
		"META-INF/container.xml",
		"OEBPS/content.opf",
		"OEBPS/nav.xhtml",
		"OEBPS/chapters/chapter-1.xhtml",
	}
	for _, name := range required {
		if !zipHas(zr, name) {
			t.Fatalf("zip missing %s", name)
		}
	}

	chapter := readZipFile(t, zr, "OEBPS/chapters/chapter-1.xhtml")
	if !bytes.Contains(chapter, []byte("Halo &lt;dunia&gt; &amp; semuanya.")) {
		t.Fatalf("chapter did not escape paragraph: %s", chapter)
	}
	if !bytes.Contains(chapter, []byte("if x &lt; y")) {
		t.Fatalf("chapter did not escape code: %s", chapter)
	}
}

func zipHas(zr *zip.Reader, name string) bool {
	for _, file := range zr.File {
		if file.Name == name {
			return true
		}
	}
	return false
}

func readZipFile(t *testing.T, zr *zip.Reader, name string) []byte {
	t.Helper()
	for _, file := range zr.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return data
	}
	t.Fatalf("missing %s", name)
	return nil
}
