package epub

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"strings"

	"aksara/internal/content"
)

type Document struct {
	Title    string
	Author   string
	Language string
	Cover    string
	Chapters []Chapter
}

type Chapter struct {
	ID     string
	Title  string
	Blocks []content.Block
}

type Builder struct{}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) Build(path string, doc content.TranslatedDocument) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	tmp := path + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create temp epub: %w", err)
	}
	defer file.Close()

	if err := Write(file, fromTranslated(doc)); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close temp epub: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit epub: %w", err)
	}
	return nil
}

func Write(w io.Writer, doc Document) error {
	if doc.Title == "" {
		doc.Title = "Untitled"
	}
	if doc.Language == "" {
		doc.Language = "id"
	}
	ensureChapterIDs(&doc)

	zw := zip.NewWriter(w)
	if err := writeMimetype(zw); err != nil {
		return err
	}
	files := map[string][]byte{
		"META-INF/container.xml": []byte(containerXML()),
		"OEBPS/content.opf":      []byte(contentOPF(doc)),
		"OEBPS/nav.xhtml":        []byte(navXHTML(doc)),
		"OEBPS/styles/style.css": []byte(stylesCSS()),
	}
	for _, chapter := range doc.Chapters {
		files["OEBPS/chapters/"+chapter.ID+".xhtml"] = []byte(chapterXHTML(doc, chapter))
	}
	if doc.Cover != "" {
		if data, err := os.ReadFile(doc.Cover); err == nil {
			files["OEBPS/images/cover"+strings.ToLower(filepath.Ext(doc.Cover))] = data
		}
	}
	for name, data := range files {
		file, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("create epub entry %s: %w", name, err)
		}
		if _, err := file.Write(data); err != nil {
			return fmt.Errorf("write epub entry %s: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("close epub: %w", err)
	}
	return nil
}

func writeMimetype(zw *zip.Writer) error {
	header := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	header.SetMode(0644)
	file, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create mimetype: %w", err)
	}
	if _, err := file.Write([]byte("application/epub+zip")); err != nil {
		return fmt.Errorf("write mimetype: %w", err)
	}
	return nil
}

func fromTranslated(doc content.TranslatedDocument) Document {
	out := Document{
		Title:    doc.Title,
		Author:   doc.Author,
		Language: doc.Language,
		Cover:    doc.Cover,
		Chapters: make([]Chapter, 0, len(doc.Chapters)),
	}
	for _, chapter := range doc.Chapters {
		out.Chapters = append(out.Chapters, Chapter{
			ID:     fmt.Sprintf("chapter-%d", chapter.Number),
			Title:  chapter.Title,
			Blocks: chapter.Blocks,
		})
	}
	return out
}

func ensureChapterIDs(doc *Document) {
	for i := range doc.Chapters {
		if doc.Chapters[i].ID == "" {
			doc.Chapters[i].ID = fmt.Sprintf("chapter-%d", i+1)
		}
		if doc.Chapters[i].Title == "" {
			doc.Chapters[i].Title = fmt.Sprintf("Bagian %d", i+1)
		}
	}
}

func containerXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`
}

func contentOPF(doc Document) string {
	id := stableID(doc.Title)
	var manifest strings.Builder
	var spine strings.Builder
	manifest.WriteString(`    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>` + "\n")
	manifest.WriteString(`    <item id="style" href="styles/style.css" media-type="text/css"/>` + "\n")
	for _, chapter := range doc.Chapters {
		manifest.WriteString(fmt.Sprintf(`    <item id="%s" href="chapters/%s.xhtml" media-type="application/xhtml+xml"/>`+"\n", xmlAttr(chapter.ID), xmlAttr(chapter.ID)))
		spine.WriteString(fmt.Sprintf(`    <itemref idref="%s"/>`+"\n", xmlAttr(chapter.ID)))
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">%s</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:language>%s</dc:language>
    <dc:creator>%s</dc:creator>
  </metadata>
  <manifest>
%s  </manifest>
  <spine>
%s  </spine>
</package>`, xmlText(id), xmlText(doc.Title), xmlText(doc.Language), xmlText(doc.Author), manifest.String(), spine.String())
}

func navXHTML(doc Document) string {
	var items strings.Builder
	for _, chapter := range doc.Chapters {
		items.WriteString(fmt.Sprintf(`      <li><a href="chapters/%s.xhtml">%s</a></li>`+"\n", xmlAttr(chapter.ID), xmlText(chapter.Title)))
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="%s">
<head>
  <title>%s</title>
  <link rel="stylesheet" type="text/css" href="styles/style.css"/>
</head>
<body>
  <nav epub:type="toc" id="toc">
    <h1>%s</h1>
    <ol>
%s    </ol>
  </nav>
</body>
</html>`, xmlAttr(doc.Language), xmlText(doc.Title), xmlText(doc.Title), items.String())
}

func chapterXHTML(doc Document, chapter Chapter) string {
	var body strings.Builder
	body.WriteString(fmt.Sprintf("  <h1>%s</h1>\n", xmlText(chapter.Title)))
	for _, block := range chapter.Blocks {
		switch block.Type {
		case content.BlockHeading:
			body.WriteString(fmt.Sprintf("  <h2>%s</h2>\n", xmlText(block.Text)))
		case content.BlockCode:
			body.WriteString(fmt.Sprintf("  <pre><code>%s</code></pre>\n", xmlText(block.Text)))
		case content.BlockListItem:
			body.WriteString(fmt.Sprintf("  <ul><li>%s</li></ul>\n", xmlText(block.Text)))
		default:
			for _, paragraph := range splitParagraphs(block.Text) {
				body.WriteString(fmt.Sprintf("  <p>%s</p>\n", xmlText(paragraph)))
			}
		}
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" lang="%s">
<head>
  <title>%s</title>
  <link rel="stylesheet" type="text/css" href="../styles/style.css"/>
</head>
<body>
%s</body>
</html>`, xmlAttr(doc.Language), xmlText(chapter.Title), body.String())
}

func stylesCSS() string {
	return `body {
  font-family: serif;
  line-height: 1.65;
}
p {
  margin: 0 0 1em;
}
pre {
  white-space: pre-wrap;
  font-family: monospace;
}
`
}

func stableID(title string) string {
	sum := sha1.Sum([]byte(title))
	return "aksara-" + hex.EncodeToString(sum[:8])
}

func xmlText(value string) string {
	return html.EscapeString(value)
}

func xmlAttr(value string) string {
	return html.EscapeString(value)
}

func splitParagraphs(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
