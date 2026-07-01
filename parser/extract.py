import sys
import json
import fitz  # PyMuPDF


def extract(pdf_path: str) -> dict:
    doc = fitz.open(pdf_path)
    metadata = doc.metadata or {}
    pages = []
    for i, page in enumerate(doc):
        text = page.get_text("text").strip()
        pages.append({"page": i + 1, "text": text})
    toc = [
        {"level": item[0], "title": item[1], "page": item[2]}
        for item in doc.get_toc()
    ]
    doc.close()
    return {
        "title": metadata.get("title") or "",
        "author": metadata.get("author") or "",
        "metadata": metadata,
        "toc": toc,
        "pages": pages,
    }


def render_cover(pdf_path: str, output_path: str):
    doc = fitz.open(pdf_path)
    page = doc[0]
    target_width = 400
    scale = target_width / page.rect.width
    pix = page.get_pixmap(matrix=fitz.Matrix(scale, scale))
    pix.save(output_path)
    doc.close()


if __name__ == "__main__":
    if len(sys.argv) >= 4 and sys.argv[1] == "--cover":
        try:
            render_cover(sys.argv[2], sys.argv[3])
        except Exception as e:
            print(json.dumps({"error": str(e)}), file=sys.stderr)
            sys.exit(1)
    elif len(sys.argv) >= 2:
        try:
            result = extract(sys.argv[1])
            print(json.dumps(result))
        except Exception as e:
            print(json.dumps({"error": str(e)}), file=sys.stderr)
            sys.exit(1)
    else:
        print(json.dumps({"error": "usage: extract.py <pdf_path> | --cover <pdf_path> <output_path>"}), file=sys.stderr)
        sys.exit(1)
