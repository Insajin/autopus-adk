package cli

import (
	"errors"
	"strings"
)

func workflowContextManagedProductCommandNames(firstPrompt string) ([2]string, error) {
	fields := strings.Fields(firstPrompt)
	if len(fields) < 2 {
		return [2]string{}, errors.New("managed OMP product command discovery input is invalid")
	}
	if fields[0] == "/auto" {
		return [2]string{"auto", "auto-" + fields[1]}, nil
	}
	if strings.HasPrefix(fields[0], "/auto-") {
		return [2]string{"auto", strings.TrimPrefix(fields[0], "/")}, nil
	}
	return [2]string{}, errors.New("managed OMP product command discovery input is invalid")
}
