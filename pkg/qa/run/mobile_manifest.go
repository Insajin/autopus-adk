package run

import (
	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func defaultArtifactsForPack(pack journey.Pack, result commandResult) []qaevidence.ArtifactRef {
	if mobileAdapter(pack.Adapter.ID) {
		return []qaevidence.ArtifactRef{
			{Kind: "sanitized_log", Path: result.StdoutPath, Publishable: true, Redaction: "text_redacted_and_scanned"},
			{Kind: "sanitized_log", Path: result.StderrPath, Publishable: true, Redaction: "text_redacted_and_scanned"},
		}
	}
	return []qaevidence.ArtifactRef{
		{Kind: "stdout", Path: result.StdoutPath, Publishable: true, Redaction: "text_redacted_and_scanned"},
		{Kind: "stderr", Path: result.StderrPath, Publishable: true, Redaction: "text_redacted_and_scanned"},
	}
}
