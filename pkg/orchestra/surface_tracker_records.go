package orchestra

import (
	"encoding/json"
	"strings"
)

type trackedSurface struct {
	Ref           string `json:"surface_ref"`
	TerminalKind  string `json:"terminal_kind,omitempty"`
	WorkspaceRef  string `json:"workspace_ref,omitempty"`
	TmuxServerRef string `json:"tmux_server_ref,omitempty"`
	rawRecord     string
}

func encodeTrackedSurface(tracked trackedSurface) string {
	if tracked.rawRecord != "" {
		return tracked.rawRecord
	}
	if tracked.TerminalKind == "" && tracked.WorkspaceRef == "" && tracked.TmuxServerRef == "" {
		return tracked.Ref
	}
	data, err := json.Marshal(tracked)
	if err != nil {
		return tracked.Ref
	}
	return string(data)
}

func decodeTrackedSurfaces(data []byte) []trackedSurface {
	var tracked []trackedSurface
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		item := trackedSurface{Ref: line}
		if strings.HasPrefix(line, "{") {
			item = trackedSurface{}
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				item.rawRecord = line
			} else if item.Ref == "" {
				item = trackedSurface{rawRecord: line}
			}
		}
		tracked = append(tracked, item)
	}
	return tracked
}

func encodeTrackedSurfaces(tracked []trackedSurface) []byte {
	lines := make([]string, 0, len(tracked))
	for _, item := range tracked {
		lines = append(lines, encodeTrackedSurface(item))
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func readTrackerRefs(path string) []string {
	tracked := readTrackedSurfaces(path)
	refs := make([]string, 0, len(tracked))
	for _, item := range tracked {
		refs = append(refs, item.Ref)
	}
	return refs
}

func writeTrackerRefs(path string, refs []string) {
	tracked := make([]trackedSurface, 0, len(refs))
	for _, ref := range refs {
		tracked = append(tracked, trackedSurface{Ref: ref})
	}
	writeTrackedSurfaces(path, tracked)
}
