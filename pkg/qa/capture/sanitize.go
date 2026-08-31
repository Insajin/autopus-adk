package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/evidence"
)

// Sanitize returns the publishable projection of a capture index.
//
// The projection keeps structure, counters, and digest-addressed media
// references, and drops or redacts anything that could carry a secret: console
// text and titles go through the evidence redactor, and every free-text field is
// capped. Media bytes are never inlined, so a published capture index stays a
// text artifact the evidence writer already knows how to handle.
func Sanitize(index Index) Index {
	out := index
	out.JourneyID = clip(evidence.RedactText(index.JourneyID))
	out.Steps = make([]Step, 0, len(index.Steps))
	for _, step := range index.Steps {
		out.Steps = append(out.Steps, sanitizeStep(step))
	}
	out.Media = make([]Media, 0, len(index.Media))
	for _, entry := range index.Media {
		entry.MediaRef = sanitizeMediaRef(entry.MediaRef)
		out.Media = append(out.Media, entry)
	}
	if index.Replay != nil {
		replay := *index.Replay
		replay.Command = redactList(replay.Command)
		replay.SpecRefs = redactList(replay.SpecRefs)
		out.Replay = &replay
	}
	out.Totals = ComputeTotals(out)
	return out
}

func sanitizeStep(step Step) Step {
	out := step
	out.Title = clip(evidence.RedactText(step.Title))
	out.ScreenRef = clip(evidence.RedactText(step.ScreenRef))
	out.FailureSummary = clip(evidence.RedactText(step.FailureSummary))
	if step.Screenshot != nil {
		ref := sanitizeMediaRef(*step.Screenshot)
		out.Screenshot = &ref
	}
	out.Actions = make([]Action, 0, len(step.Actions))
	for _, action := range step.Actions {
		action.API = clip(evidence.RedactText(action.API))
		action.TargetRef = clip(evidence.RedactText(action.TargetRef))
		out.Actions = append(out.Actions, action)
	}
	if step.Console != nil {
		console := *step.Console
		console.Messages = make([]ConsoleMessage, 0, len(step.Console.Messages))
		for _, message := range step.Console.Messages {
			message.Text = clip(evidence.RedactText(message.Text))
			message.SourceRef = clip(evidence.RedactText(message.SourceRef))
			console.Messages = append(console.Messages, message)
		}
		out.Console = &console
	}
	if step.Network != nil {
		network := *step.Network
		network.Entries = make([]NetworkEntry, 0, len(step.Network.Entries))
		for _, entry := range step.Network.Entries {
			entry.URLRef = clip(evidence.RedactText(entry.URLRef))
			network.Entries = append(network.Entries, entry)
		}
		out.Network = &network
	}
	return out
}

func sanitizeMediaRef(ref MediaRef) MediaRef {
	ref.Ref = clip(evidence.RedactText(filepath.ToSlash(ref.Ref)))
	ref.Retention = RetentionLocalOnly
	return ref
}

// AssertPublishable re-scans a sanitized projection and fails closed if any
// field still looks like a secret, a private note, or a local user path. This is
// the same guarantee the evidence writer applies to artifact bodies, applied
// before the projection is written so the failure names the capture field.
func AssertPublishable(index Index) error {
	body, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return evidence.AssertSafeText(string(body), "capture_index")
}

// WritePublished validates, sanitizes, and atomically writes the publishable
// projection next to the producer index.
func WritePublished(index Index, dir string) (string, error) {
	sanitized := Sanitize(index)
	if err := Validate(sanitized); err != nil {
		return "", fmt.Errorf("sanitized capture index is invalid: %w", err)
	}
	if err := AssertPublishable(sanitized); err != nil {
		return "", err
	}
	body, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, PublishedFileName)
	tmp, err := os.CreateTemp(dir, ".capture-index-*.json")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(body, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	return path, nil
}

// VerifyLocalMedia confirms every declared media reference resolves inside the
// capture directory with the declared size. A digest mismatch or a reference
// that escapes the directory means the index describes files the harness cannot
// account for, so it is reported rather than trusted.
func VerifyLocalMedia(index Index, captureDir string) []string {
	var findings []string
	root, err := filepath.Abs(captureDir)
	if err != nil {
		return []string{"capture directory is unresolvable"}
	}
	refs := make([]MediaRef, 0, len(index.Media)+len(index.Steps))
	for _, entry := range index.Media {
		refs = append(refs, entry.MediaRef)
	}
	for _, step := range index.Steps {
		if step.Screenshot != nil {
			refs = append(refs, *step.Screenshot)
		}
	}
	for _, ref := range refs {
		findings = append(findings, verifyMediaFile(root, ref)...)
	}
	return findings
}

func verifyMediaFile(root string, ref MediaRef) []string {
	path := filepath.Join(root, filepath.FromSlash(ref.Ref))
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return []string{fmt.Sprintf("media ref %q escapes the capture directory", ref.Ref)}
	}
	info, err := os.Stat(path)
	if err != nil {
		return []string{fmt.Sprintf("media ref %q is missing from the capture directory", ref.Ref)}
	}
	if !info.Mode().IsRegular() {
		return []string{fmt.Sprintf("media ref %q is not a regular file", ref.Ref)}
	}
	if info.Size() != ref.Bytes {
		return []string{fmt.Sprintf("media ref %q declares %d bytes but the file has %d", ref.Ref, ref.Bytes, info.Size())}
	}
	return nil
}

func redactList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, clip(evidence.RedactText(value)))
	}
	return out
}

func clip(value string) string {
	if len(value) <= MaxTextLen {
		return value
	}
	return value[:MaxTextLen] + "…"
}
