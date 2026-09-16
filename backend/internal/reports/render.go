package reports

import (
	"bytes"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// todayStr stamps report generation time (UTC, date precision is enough).
func todayStr() string { return time.Now().UTC().Format("2006-01-02 15:04") }

// ToMarkdown renders the report as Markdown.
func (r Report) ToMarkdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — FlowSight Report (%s)\n\n", r.Ticker, r.GeneratedAt)
	for _, s := range r.Sections {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", s.Name, s.Body)
		if len(s.Citations) > 0 {
			b.WriteString("Citations:\n")
			for _, c := range s.Citations {
				fmt.Fprintf(&b, "- %s %s @ %s\n", c.Endpoint, c.Ticker, c.SnapshotAt)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// ToHTML renders the report as a standalone page. All dynamic values are
// HTML-escaped — bodies come from stored snapshots/LLM and must never be able
// to inject script into the exported file.
func (r Report) ToHTML() string {
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>")
	b.WriteString(esc(r.Ticker) + " — FlowSight Report</title></head><body>")
	fmt.Fprintf(&b, "<h1>%s — FlowSight Report (%s)</h1>", esc(r.Ticker), esc(r.GeneratedAt))
	for _, s := range r.Sections {
		fmt.Fprintf(&b, "<h2>%s</h2><p>%s</p>", esc(s.Name), esc(s.Body))
		if len(s.Citations) > 0 {
			b.WriteString("<ul>")
			for _, c := range s.Citations {
				fmt.Fprintf(&b, "<li>%s %s @ %s</li>", esc(c.Endpoint), esc(c.Ticker), esc(c.SnapshotAt))
			}
			b.WriteString("</ul>")
		}
	}
	b.WriteString("</body></html>")
	return b.String()
}

// ToPDF renders the report server-side (pure Go, no system deps).
// Font: DejaVuSans (Unicode) instead of core Helvetica so Indonesian text
// (Em-dash, quotes, Rp, accents) renders correctly — latin1() was turning
// every non-Latin rune into '?'.
func (r Report) ToPDF() ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetFontLocation("/usr/share/fonts/truetype/dejavu/")
	pdf.AddUTF8Font("DejaVu", "", "DejaVuSans.ttf")
	pdf.AddUTF8Font("DejaVu", "B", "DejaVuSans-Bold.ttf")
	pdf.AddPage()
	pdf.SetFont("DejaVu", "B", 14)
	pdf.Cell(0, 10, r.Ticker+" — FlowSight Report")
	pdf.Ln(12)
	pdf.SetFont("DejaVu", "", 10)
	pdf.Cell(0, 6, "Generated "+r.GeneratedAt)
	pdf.Ln(8)
	for _, s := range r.Sections {
		pdf.SetFont("DejaVu", "B", 12)
		pdf.Cell(0, 8, s.Name)
		pdf.Ln(8)
		pdf.SetFont("DejaVu", "", 10)
		pdf.MultiCell(0, 5, s.Body, "", "", false)
		if len(s.Citations) > 0 {
			pdf.SetFont("DejaVu", "", 8)
			for _, c := range s.Citations {
				pdf.MultiCell(0, 4, c.Endpoint+" "+c.Ticker+" @ "+c.SnapshotAt, "", "", false)
			}
		}
		pdf.Ln(4)
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
