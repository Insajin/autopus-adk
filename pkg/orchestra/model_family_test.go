package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitModelSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		selector     string
		wantModel    string
		wantThinking string
		wantOK       bool
	}{
		{name: "model and thinking", selector: "openai-codex/gpt-6-astra:max", wantModel: "openai-codex/gpt-6-astra", wantThinking: "max", wantOK: true},
		{name: "model only", selector: "anthropic/claude-fable-5-1", wantModel: "anthropic/claude-fable-5-1", wantOK: true},
		{name: "empty", selector: "", wantOK: false},
		{name: "missing slash", selector: "gpt-6-astra", wantOK: false},
		{name: "missing provider", selector: "/gpt-6-astra", wantOK: false},
		{name: "missing model", selector: "openai-codex/", wantOK: false},
		{name: "missing thinking", selector: "openai-codex/gpt-6-astra:", wantOK: false},
		{name: "extra slash", selector: "openai/codex/gpt-6-astra", wantOK: false},
		{name: "extra colon", selector: "openai/gpt-6:max:extra", wantOK: false},
		{name: "thinking delimiter before model", selector: "openai:high/gpt-6", wantOK: false},
		{name: "whitespace", selector: " openai/gpt-6", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model, thinking, ok := SplitModelSelector(tt.selector)
			assert.Equal(t, tt.wantModel, model)
			assert.Equal(t, tt.wantThinking, thinking)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestModelFamilyForSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		selector string
		want     string
	}{
		{selector: "anthropic/claude-fable-5-1:max", want: "anthropic"},
		{selector: "openai/gpt-6-astra", want: "openai"},
		{selector: "openai-codex/gpt-6-astra:max", want: "openai"},
		{selector: "google/gemini-3.1-pro", want: "google"},
		{selector: "google-antigravity/gemini-3.1-pro:high", want: "google"},
		{selector: "google-vertex/gemini-3.1-pro", want: "google"},
		{selector: "custom/model:deep", want: "custom"},
		{selector: "", want: ""},
		{selector: "malformed", want: ""},
		{selector: "openai/", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.selector, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ModelFamilyForSelector(tt.selector))
		})
	}
}
