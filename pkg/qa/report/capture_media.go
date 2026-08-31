package report

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
)

// Embedded media caps. A screenshot is the largest thing a report can inline and
// the file must stay openable in a browser, so one image and the whole document
// are both bounded. Base64 grows the document by about a third past these
// numbers, which is why the report cap is well under a browser's comfort limit.
const (
	maxEmbeddedImageBytes  = 2 << 20
	maxEmbeddedReportBytes = 24 << 20
)

// imageMediaTypes is the closed set of raster formats that may be inlined.
// Extension, not sniffing, decides: the producer declared the format, and an
// unexpected one is a contract problem rather than something to guess at.
var imageMediaTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
}

// mediaBudget carries the opt-in decision and the whole-report allowance. A nil
// budget means "never embed", which is what every journey-level media reference
// and every default report gets.
type mediaBudget struct {
	embed     bool
	bytesLeft int64
}

func newMediaBudget(embed bool) *mediaBudget {
	return &mediaBudget{embed: embed, bytesLeft: maxEmbeddedReportBytes}
}

// mediaView projects one media reference. Bytes are inlined only when the caller
// opted in and the file on disk is exactly the file the index describes, so an
// embedded image can never disagree with the digest printed beside it.
func mediaView(kind, dir string, ref capture.MediaRef, budget *mediaBudget) CaptureMediaView {
	view := CaptureMediaView{
		Kind:      orUnknown(kind),
		Ref:       captureText(filepath.ToSlash(ref.Ref)),
		Digest:    ref.Digest,
		Bytes:     ref.Bytes,
		Width:     ref.Width,
		Height:    ref.Height,
		Retention: orUnknown(ref.Retention),
	}
	if budget == nil || !budget.embed {
		return view
	}
	body, mediaType, reason := readEmbeddableMedia(dir, ref, budget)
	if reason != "" {
		view.MediaError = reason
		return view
	}
	view.DataURI = "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(body)
	view.Embedded = true
	budget.bytesLeft -= int64(len(body))
	return view
}

// readEmbeddableMedia applies every embedding gate in order and returns a stable
// refusal token instead of an error, so the refusal renders next to the frame.
func readEmbeddableMedia(dir string, ref capture.MediaRef, budget *mediaBudget) ([]byte, string, string) {
	mediaType, ok := imageMediaTypes[strings.ToLower(filepath.Ext(ref.Ref))]
	if !ok {
		return nil, "", "unsupported_media_format"
	}
	path, ok := confinedMediaPath(dir, ref.Ref)
	if !ok {
		return nil, "", "unsafe_media_ref"
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", "missing_media_file"
	}
	if !info.Mode().IsRegular() {
		return nil, "", "not_regular_file"
	}
	if info.Size() > maxEmbeddedImageBytes {
		return nil, "", "media_too_large"
	}
	if info.Size() != ref.Bytes {
		return nil, "", "declared_bytes_mismatch"
	}
	if info.Size() > budget.bytesLeft {
		return nil, "", "embed_budget_exhausted"
	}
	body, err := readMediaFile(path, maxEmbeddedImageBytes)
	if err != nil {
		return nil, "", "unreadable_media_file"
	}
	// A digest mismatch means the index does not describe the bytes on disk, so
	// the frame would show an image no evidence vouches for.
	if mediaDigest(body) != ref.Digest {
		return nil, "", "digest_mismatch"
	}
	return body, mediaType, ""
}

// confinedMediaPath resolves a capture-directory-relative ref and refuses
// anything that leaves the directory, including via a symlink, because the
// producer controls both the ref and the files it points at.
func confinedMediaPath(dir, ref string) (string, bool) {
	text := strings.TrimSpace(ref)
	if text == "" || strings.ContainsAny(text, "\x00\r\n\t") {
		return "", false
	}
	if filepath.IsAbs(text) || strings.HasPrefix(text, "\\") || strings.Contains(text, ":\\") {
		return "", false
	}
	root, err := realPath(dir)
	if err != nil {
		return "", false
	}
	resolved, err := realPath(filepath.Join(root, filepath.FromSlash(text)))
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == "." || escapesRoot(rel) {
		return "", false
	}
	return resolved, true
}

func readMediaFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, limit))
}

func mediaDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}
