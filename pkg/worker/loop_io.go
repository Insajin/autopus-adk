package worker

import (
	"io"
	"log"
	"strings"
)

// StdinWriter wraps an io.WriteCloser to keep the stdin pipe open
// after the initial prompt is written. This enables mid-session
// message injection (e.g., budget warnings).
type StdinWriter struct {
	pipe io.WriteCloser
}

// NewStdinWriter creates a StdinWriter wrapping the given pipe.
func NewStdinWriter(pipe io.WriteCloser) *StdinWriter {
	return &StdinWriter{pipe: pipe}
}

// WritePrompt sends the initial prompt to the subprocess stdin.
// Unlike the previous implementation, the pipe is NOT closed after writing.
func (sw *StdinWriter) WritePrompt(prompt string) error {
	_, err := io.Copy(sw.pipe, strings.NewReader(prompt))
	return err
}

// Write implements io.Writer for injecting messages into stdin.
func (sw *StdinWriter) Write(p []byte) (int, error) {
	return sw.pipe.Write(p)
}

// Close closes the underlying pipe.
func (sw *StdinWriter) Close() error {
	return sw.pipe.Close()
}

func recordAuditEvent(w io.Writer, evt AuditEvent, logger LogBuffer) {
	if err := writeResilientAuditEvent(w, evt, logger); err != nil {
		log.Printf("[worker] audit event write failed: %v", err)
	}
}
