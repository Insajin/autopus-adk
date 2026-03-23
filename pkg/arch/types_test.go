package arch_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/arch"
)

func TestValidationError_Error(t *testing.T) {
	t.Parallel()
	e := arch.ValidationError{Field: "file.go", Message: "exceeds line limit"}
	assert.Equal(t, "exceeds line limit", e.Error())
}

func TestValidationError_ErrorEmpty(t *testing.T) {
	t.Parallel()
	e := arch.ValidationError{}
	assert.Equal(t, "", e.Error())
}
