package template

import (
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestTruncateToBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		content  string
		maxBytes int
		want     string
	}{
		{
			name:     "empty string",
			content:  "",
			maxBytes: 10,
			want:     "",
		},
		{
			name:     "fits within limit",
			content:  "hello",
			maxBytes: 10,
			want:     "hello",
		},
		{
			name:     "exact fit",
			content:  "hello",
			maxBytes: 5,
			want:     "hello",
		},
		{
			name:     "truncate ascii",
			content:  "hello world",
			maxBytes: 5,
			want:     "hello",
		},
		{
			name:     "zero max bytes",
			content:  "hello",
			maxBytes: 0,
			want:     "",
		},
		{
			name:     "negative max bytes",
			content:  "hello",
			maxBytes: -1,
			want:     "",
		},
		{
			name:     "truncate mid utf8 2-byte",
			content:  "café", // é is 2 bytes (0xC3 0xA9)
			maxBytes: 4,      // "caf" = 3 bytes, é starts at byte 3
			want:     "caf",  // can't fit é, so truncate before it
		},
		{
			name:     "truncate mid utf8 3-byte",
			content:  "ab한글", // 한 is 3 bytes
			maxBytes: 3,      // "ab" = 2 bytes, 한 starts at byte 2
			want:     "ab",
		},
		{
			name:     "truncate mid utf8 4-byte emoji",
			content:  "hi🐙x", // 🐙 is 4 bytes
			maxBytes: 4,      // "hi" = 2 bytes, 🐙 starts at byte 2
			want:     "hi",
		},
		{
			name:     "full utf8 char fits",
			content:  "café",
			maxBytes: 5, // "caf" + é = 5 bytes
			want:     "café",
		},
		{
			name:     "max bytes 1 with multibyte start",
			content:  "한글",
			maxBytes: 1,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := TruncateToBytes(tt.content, tt.maxBytes)
			assert.Equal(t, tt.want, got)
			// Result must always be valid UTF-8.
			if len(got) > 0 {
				assert.True(t, utf8.ValidString(got), "result should be valid UTF-8")
			}
		})
	}
}

func TestMapPermission(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		claudeMode string
		platform   string
		want       string
	}{
		// Codex mappings
		{"codex plan", "plan", "codex", "on-request"},
		{"codex act", "act", "codex", "auto"},
		{"codex bypass", "bypass", "codex", "never"},
		// Gemini mappings
		{"gemini plan", "plan", "gemini-cli", "plan"},
		{"gemini act", "act", "gemini-cli", "auto_edit"},
		{"gemini bypass", "bypass", "gemini-cli", "yolo"},
		// Unknown platform
		{"unknown platform", "plan", "unknown", ""},
		// Unknown mode
		{"unknown mode codex", "unknown", "codex", ""},
		{"unknown mode gemini", "unknown", "gemini-cli", ""},
		// Empty inputs
		{"empty mode", "", "codex", ""},
		{"empty platform", "plan", "", ""},
		{"both empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MapPermission(tt.claudeMode, tt.platform)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSkillList(t *testing.T) {
	t.Parallel()

	t.Run("nil config", func(t *testing.T) {
		t.Parallel()
		got := SkillList(nil)
		assert.Nil(t, got)
	})

	t.Run("empty category weights", func(t *testing.T) {
		t.Parallel()
		cfg := &config.HarnessConfig{}
		got := SkillList(cfg)
		assert.Nil(t, got)
	})

	t.Run("with categories", func(t *testing.T) {
		t.Parallel()
		cfg := &config.HarnessConfig{
			Skills: config.SkillsConf{
				CategoryWeights: map[string]int{
					"workflow": 10,
					"quality":  5,
					"explore":  3,
				},
			},
		}
		got := SkillList(cfg)
		assert.Len(t, got, 3)

		// Sort for deterministic comparison.
		names := make([]string, len(got))
		for i, s := range got {
			names[i] = s.Name
		}
		sort.Strings(names)
		assert.Equal(t, []string{"explore", "quality", "workflow"}, names)
	})

	t.Run("single category", func(t *testing.T) {
		t.Parallel()
		cfg := &config.HarnessConfig{
			Skills: config.SkillsConf{
				CategoryWeights: map[string]int{
					"debug": 1,
				},
			},
		}
		got := SkillList(cfg)
		assert.Len(t, got, 1)
		assert.Equal(t, "debug", got[0].Name)
	})
}

func TestTruncateToBytes_LargeContent(t *testing.T) {
	t.Parallel()
	// Build a large string with mixed ASCII and multi-byte chars.
	content := strings.Repeat("a한b", 1000) // each repeat = 1+3+1 = 5 bytes
	got := TruncateToBytes(content, 100)
	assert.LessOrEqual(t, len(got), 100)
	assert.Greater(t, len(got), 0)
}
