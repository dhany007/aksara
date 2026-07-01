package extract

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"aksara/internal/book"
	"aksara/internal/content"
)

type Extractor struct {
	PythonBin    string
	ParserScript string
}

func New(pythonBin, parserScript string) *Extractor {
	return &Extractor{PythonBin: pythonBin, ParserScript: parserScript}
}

func (e *Extractor) Extract(ctx context.Context, input book.Book, cacheDir string) (content.SourceDocument, error) {
	switch input.Format {
	case book.FormatPDF:
		return e.extractPDF(ctx, input, cacheDir)
	case book.FormatEPUB:
		return e.extractEPUB(input, cacheDir)
	default:
		return content.SourceDocument{}, fmt.Errorf("unsupported book format %q", input.Format)
	}
}

type pdfExtractResult struct {
	Title    string            `json:"title"`
	Author   string            `json:"author"`
	Metadata map[string]string `json:"metadata"`
	Pages    []struct {
		Page int    `json:"page"`
		Text string `json:"text"`
	} `json:"pages"`
}

func (e *Extractor) extractPDF(ctx context.Context, input book.Book, cacheDir string) (content.SourceDocument, error) {
	cmd := exec.CommandContext(ctx, e.PythonBin, e.ParserScript, input.Path)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return content.SourceDocument{}, fmt.Errorf("python exit %d: %s", exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return content.SourceDocument{}, err
	}

	var result pdfExtractResult
	if err := json.Unmarshal(out, &result); err != nil {
		return content.SourceDocument{}, fmt.Errorf("parse pdf extractor output: %w", err)
	}

	source := content.SourceDocument{
		Title:  firstNonEmpty(result.Title, result.Metadata["title"], input.Title),
		Author: firstNonEmpty(result.Author, result.Metadata["author"]),
	}
	for _, page := range result.Pages {
		text := strings.TrimSpace(page.Text)
		if text == "" {
			continue
		}
		source.Sections = append(source.Sections, content.SourceSection{
			Title: fmt.Sprintf("Halaman %d", page.Page),
			Start: page.Page,
			End:   page.Page,
			Text:  text,
		})
	}

	coverPath := filepath.Join(cacheDir, "cover.jpg")
	if err := os.MkdirAll(cacheDir, 0755); err == nil {
		coverCmd := exec.CommandContext(ctx, e.PythonBin, e.ParserScript, "--cover", input.Path, coverPath)
		if err := coverCmd.Run(); err == nil {
			source.Cover = coverPath
		}
	}

	return source, nil
}

func (e *Extractor) extractEPUB(input book.Book, cacheDir string) (content.SourceDocument, error) {
	zr, err := zip.OpenReader(input.Path)
	if err != nil {
		return content.SourceDocument{}, fmt.Errorf("open epub: %w", err)
	}
	defer zr.Close()

	rootfile, err := epubRootfile(&zr.Reader)
	if err != nil {
		return content.SourceDocument{}, err
	}
	opfData, err := readZip(&zr.Reader, rootfile)
	if err != nil {
		return content.SourceDocument{}, err
	}
	var opf packageFile
	if err := xml.Unmarshal(opfData, &opf); err != nil {
		return content.SourceDocument{}, fmt.Errorf("parse opf: %w", err)
	}
	baseDir := filepath.ToSlash(filepath.Dir(rootfile))
	if baseDir == "." {
		baseDir = ""
	}

	manifest := map[string]manifestItem{}
	for _, item := range opf.Manifest.Items {
		manifest[item.ID] = item
	}

	source := content.SourceDocument{
		Title:  firstNonEmpty(opf.Metadata.Title, input.Title),
		Author: opf.Metadata.Creator,
	}
	if cover := findCoverItem(opf.Manifest.Items); cover.Href != "" {
		if path, err := writeEPUBAsset(&zr.Reader, baseDir, cover.Href, cacheDir, "cover"+filepath.Ext(cover.Href)); err == nil {
			source.Cover = path
		}
	}

	for i, ref := range opf.Spine.ItemRefs {
		item, ok := manifest[ref.IDRef]
		if !ok || !isXHTML(item.MediaType) {
			continue
		}
		data, err := readZip(&zr.Reader, joinZip(baseDir, item.Href))
		if err != nil {
			return content.SourceDocument{}, err
		}
		text := strings.TrimSpace(textFromXHTML(data))
		if text == "" {
			continue
		}
		source.Sections = append(source.Sections, content.SourceSection{
			Title: firstNonEmpty(titleFromXHTML(data), fmt.Sprintf("Chapter %d", i+1)),
			Start: i + 1,
			End:   i + 1,
			Text:  text,
		})
	}
	return source, nil
}

type containerFile struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type packageFile struct {
	Metadata struct {
		Title   string `xml:"title"`
		Creator string `xml:"creator"`
	} `xml:"metadata"`
	Manifest struct {
		Items []manifestItem `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		ItemRefs []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

type manifestItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

func epubRootfile(zr *zip.Reader) (string, error) {
	data, err := readZip(zr, "META-INF/container.xml")
	if err != nil {
		return "", err
	}
	var container containerFile
	if err := xml.Unmarshal(data, &container); err != nil {
		return "", fmt.Errorf("parse container.xml: %w", err)
	}
	if len(container.Rootfiles) == 0 || container.Rootfiles[0].FullPath == "" {
		return "", fmt.Errorf("epub container has no rootfile")
	}
	return container.Rootfiles[0].FullPath, nil
}

func readZip(zr *zip.Reader, name string) ([]byte, error) {
	name = filepath.ToSlash(name)
	for _, file := range zr.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open epub entry %s: %w", name, err)
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("read epub entry %s: %w", name, err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("epub entry not found: %s", name)
}

func findCoverItem(items []manifestItem) manifestItem {
	for _, item := range items {
		if strings.Contains(item.Properties, "cover-image") {
			return item
		}
	}
	for _, item := range items {
		if strings.EqualFold(item.ID, "cover") && strings.HasPrefix(item.MediaType, "image/") {
			return item
		}
	}
	return manifestItem{}
}

func writeEPUBAsset(zr *zip.Reader, baseDir, href, cacheDir, name string) (string, error) {
	data, err := readZip(zr, joinZip(baseDir, href))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	path := filepath.Join(cacheDir, name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write epub asset: %w", err)
	}
	return path, nil
}

func isXHTML(mediaType string) bool {
	return mediaType == "application/xhtml+xml" || mediaType == "text/html"
}

func joinZip(baseDir, href string) string {
	if baseDir == "" {
		return filepath.ToSlash(href)
	}
	return filepath.ToSlash(filepath.Join(baseDir, href))
}

func textFromXHTML(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var out strings.Builder
	var skip string
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch tok := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(tok.Name.Local)
			if name == "script" || name == "style" {
				skip = name
			}
			if isBlockElement(name) && out.Len() > 0 {
				out.WriteString("\n\n")
			}
		case xml.EndElement:
			if skip == strings.ToLower(tok.Name.Local) {
				skip = ""
			}
		case xml.CharData:
			if skip != "" {
				continue
			}
			text := strings.TrimSpace(string(tok))
			if text != "" {
				if out.Len() > 0 && !strings.HasSuffix(out.String(), "\n\n") {
					out.WriteByte(' ')
				}
				out.WriteString(html.UnescapeString(text))
			}
		}
	}
	if out.Len() > 0 {
		return strings.TrimSpace(out.String())
	}
	return stripTagsFallback(data)
}

func titleFromXHTML(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var inHeading bool
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		switch tok := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(tok.Name.Local)
			inHeading = name == "h1" || name == "h2"
		case xml.EndElement:
			inHeading = false
		case xml.CharData:
			if inHeading {
				if title := strings.TrimSpace(string(tok)); title != "" {
					return html.UnescapeString(title)
				}
			}
		}
	}
}

func stripTagsFallback(data []byte) string {
	var out strings.Builder
	inTag := false
	for _, r := range string(data) {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
			out.WriteByte(' ')
		default:
			if !inTag {
				out.WriteRune(r)
			}
		}
	}
	return strings.Join(strings.Fields(html.UnescapeString(out.String())), " ")
}

func isBlockElement(name string) bool {
	switch name {
	case "p", "div", "section", "article", "chapter", "h1", "h2", "h3", "h4", "h5", "h6", "li", "br":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
