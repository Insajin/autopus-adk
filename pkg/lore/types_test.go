package lore_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/lore"
)

func TestValidationError_Error(t *testing.T) {
	t.Parallel()
	e := lore.ValidationError{Message: "invalid commit type"}
	assert.Equal(t, "invalid commit type", e.Error())
}

func TestValidationError_ErrorEmpty(t *testing.T) {
	t.Parallel()
	e := lore.ValidationError{}
	assert.Equal(t, "", e.Error())
}
