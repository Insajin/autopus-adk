package cli

import (
	"errors"
	"strings"
)

var errWorkflowContextManagedProductCommand = errors.New(
	"managed OMP product command discovery input is invalid",
)

func workflowContextManagedProductCommandNames(firstPrompt string) ([2]string, error) {
	route, err := workflowContextManagedProductRoute(firstPrompt)
	if err != nil {
		return [2]string{}, err
	}
	if route == "" {
		return [2]string{"auto", "auto"}, nil
	}
	return [2]string{"auto", "auto-" + route}, nil
}

func workflowContextManagedProductRoute(firstPrompt string) (string, error) {
	if firstPrompt == "" || firstPrompt != strings.TrimSpace(firstPrompt) ||
		strings.ContainsAny(firstPrompt, "\r\n") {
		return "", errWorkflowContextManagedProductCommand
	}
	fields := strings.Fields(firstPrompt)
	if len(fields) == 0 {
		return "", errWorkflowContextManagedProductCommand
	}
	if fields[0] == "/auto" {
		return workflowContextManagedProductRoutedCommand(fields)
	}
	if !strings.HasPrefix(fields[0], "/auto-") {
		return "", errWorkflowContextManagedProductCommand
	}
	route := strings.TrimPrefix(fields[0], "/auto-")
	if !workflowContextManagedProductRouteSupported(route) {
		return "", errWorkflowContextManagedProductCommand
	}
	if err := validateWorkflowContextManagedProductGlobalFlags(fields[1:]); err != nil {
		return "", err
	}
	return route, nil
}

func workflowContextManagedProductRoutedCommand(fields []string) (string, error) {
	if len(fields) == 1 {
		return "", nil
	}
	for index := 1; index < len(fields); {
		width, global, err := workflowContextManagedProductGlobalFlagWidth(fields, index)
		if err != nil {
			return "", err
		}
		if global {
			index += width
			continue
		}
		route := fields[index]
		if strings.HasPrefix(route, "-") || !workflowContextManagedProductRouteSupported(route) {
			return "", errWorkflowContextManagedProductCommand
		}
		if err := validateWorkflowContextManagedProductGlobalFlags(fields[index+1:]); err != nil {
			return "", err
		}
		return route, nil
	}
	return "", errWorkflowContextManagedProductCommand
}

func validateWorkflowContextManagedProductGlobalFlags(fields []string) error {
	for index := 0; index < len(fields); index++ {
		width, global, err := workflowContextManagedProductGlobalFlagWidth(fields, index)
		if err != nil {
			return err
		}
		if global {
			index += width - 1
		}
	}
	return nil
}

func workflowContextManagedProductGlobalFlagWidth(
	fields []string, index int,
) (int, bool, error) {
	token := fields[index]
	switch token {
	case "--auto", "--loop", "--multi", "--team", "--solo", "--think", "--ultrathink", "--workflow":
		return 1, true, nil
	case "--quality", "--model", "--variant":
		if index+1 >= len(fields) || strings.HasPrefix(fields[index+1], "--") {
			return 0, true, errWorkflowContextManagedProductCommand
		}
		return 2, true, nil
	}
	switch {
	case strings.HasPrefix(token, "--quality="),
		strings.HasPrefix(token, "--model="),
		strings.HasPrefix(token, "--variant="):
		if strings.HasSuffix(token, "=") {
			return 0, true, errWorkflowContextManagedProductCommand
		}
		return 1, true, nil
	case strings.HasPrefix(token, "--auto="),
		strings.HasPrefix(token, "--loop="),
		strings.HasPrefix(token, "--multi="),
		strings.HasPrefix(token, "--team="),
		strings.HasPrefix(token, "--solo="),
		strings.HasPrefix(token, "--think="),
		strings.HasPrefix(token, "--ultrathink="),
		strings.HasPrefix(token, "--workflow="):
		return 0, true, errWorkflowContextManagedProductCommand
	default:
		return 0, false, nil
	}
}

func workflowContextManagedProductRouteSupported(route string) bool {
	switch route {
	case "setup", "status", "goal", "update", "plan", "go", "fix", "review", "sync",
		"idea", "map", "why", "verify", "secure", "test", "qa", "dev", "canary", "doctor":
		return true
	default:
		return false
	}
}
