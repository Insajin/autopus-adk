package report

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed assets/report.html.tmpl assets/report_capture.html.tmpl assets/report.css
var assets embed.FS

// renderData wraps the projection with the inlined stylesheet. The stylesheet is
// marked template.CSS because it is a build-time asset, never evidence content.
type renderData struct {
	Report
	CSS template.CSS
}

var reportTemplate = template.Must(template.New("report.html.tmpl").Funcs(template.FuncMap{
	"nz":     nonEmpty,
	"dur":    humanDuration,
	"pctf":   formatPercent,
	"bytes":  humanBytes,
	"digest": shortDigest,
	"imgsrc": imageSource,
}).ParseFS(assets, "assets/report.html.tmpl", "assets/report_capture.html.tmpl"))

// Render writes the self-contained HTML report. The renderer touches no files:
// every value comes from the already-validated projection, and html/template
// escapes it for the exact HTML context it lands in.
func Render(w io.Writer, report Report) error {
	css, err := assets.ReadFile("assets/report.css")
	if err != nil {
		return err
	}
	return reportTemplate.Execute(w, renderData{Report: report, CSS: template.CSS(css)})
}

// WriteFile renders the report to path, creating parent directories. The file is
// written atomically so a half-rendered report is never left behind.
func WriteFile(report Report, path string) error {
	var buf bytes.Buffer
	if err := Render(&buf, report); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".qa-report-*.html")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// DefaultOutputPath places the report beside the run index it describes, so the
// report ships with the evidence directory it was built from.
func DefaultOutputPath(runIndexPath string) string {
	if runIndexPath == "" {
		return DefaultReportFile
	}
	return filepath.Join(filepath.Dir(runIndexPath), DefaultReportFile)
}

func nonEmpty(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func humanDuration(ms int64) string {
	if ms <= 0 {
		return "—"
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("%dms", ms)
	}
	if d < time.Minute {
		return strconv.FormatFloat(d.Seconds(), 'f', 1, 64) + "s"
	}
	return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
}

func formatPercent(value float64) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func humanBytes(size int64) string {
	switch {
	case size <= 0:
		return "0 B"
	case size < 1024:
		return fmt.Sprintf("%d B", size)
	case size < 1024*1024:
		return strconv.FormatFloat(float64(size)/1024, 'f', 1, 64) + " KB"
	default:
		return strconv.FormatFloat(float64(size)/(1024*1024), 'f', 1, 64) + " MB"
	}
}

// shortDigest keeps a content digest identifiable inside a narrow filmstrip tile
// without wrapping the tile. The full digest stays in the JSON projection.
func shortDigest(value string) string {
	// "sha256:" plus twelve hex characters is unambiguous for a local capture.
	const keep = len("sha256:") + 12
	if len(value) <= keep {
		return value
	}
	return value[:keep] + "…"
}

// imageSource re-types an embedded data URL for a src attribute. html/template
// rejects every non-http scheme there, so the value has to be admitted
// explicitly; anything that is not the base64 image URL the capture projection
// produced collapses to an empty attribute instead of being trusted.
func imageSource(value string) template.URL {
	if !strings.HasPrefix(value, "data:image/") || !strings.Contains(value, ";base64,") {
		return ""
	}
	if strings.ContainsAny(value, "\"'<>\\ \t\r\n") {
		return ""
	}
	return template.URL(value)
}
