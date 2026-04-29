package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreviewCategoryForPath_DesignImportsAreRuntimeState(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "runtime_state", previewCategoryForPath(".autopus/design/imports/abc123/metadata.json"))
	assert.Equal(t, "runtime_state", previewCategoryForPath(".autopus/design/imports/abc123/content.md"))
}
