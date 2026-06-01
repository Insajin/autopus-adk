package run

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func evaluateGUIScreenMatrix(doc map[string]any, matrix []journey.GUIScreenMatrixRow) ([]string, []string) {
	if len(matrix) == 0 {
		return nil, nil
	}
	visited := guiVisitedScreenTokens(doc)
	actions := guiCompletedActionTokens(doc)
	missingScreens := []string{}
	missingActions := []string{}
	for _, screen := range matrix {
		if !guiScreenVisited(screen, visited) {
			missingScreens = append(missingScreens, guiScreenLabel(screen))
		}
		for _, action := range screen.RequiredActions {
			if !guiActionCompleted(screen, action, actions) {
				missingActions = append(missingActions, guiScreenLabel(screen)+":"+strings.TrimSpace(action))
			}
		}
	}
	return missingScreens, missingActions
}

func guiVisitedScreenTokens(doc map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, key := range []string{"visited_screens", "screens", "routes", "visited_routes"} {
		addGUIScreenTokens(out, doc[key])
	}
	return out
}

func addGUIScreenTokens(out map[string]bool, value any) {
	switch typed := value.(type) {
	case string:
		addGUIToken(out, typed)
	case []any:
		for _, item := range typed {
			addGUIScreenTokens(out, item)
		}
	case map[string]any:
		if visited, ok := typed["visited"].(bool); ok && !visited {
			return
		}
		for _, key := range []string{"id", "screen_id", "name", "path", "route", "url"} {
			addGUIToken(out, stringValue(typed[key]))
		}
	}
}

func guiCompletedActionTokens(doc map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, key := range []string{"completed_actions", "screen_actions", "interactions", "actions", "controls"} {
		addGUIActionTokens(out, doc[key])
	}
	return out
}

func addGUIActionTokens(out map[string]bool, value any) {
	switch typed := value.(type) {
	case string:
		addGUIToken(out, typed)
	case []any:
		for _, item := range typed {
			addGUIActionTokens(out, item)
		}
	case map[string]any:
		if !guiActionStatusCounts(typed) {
			return
		}
		action := firstText(typed, "action_id", "action", "id", "name", "control", "label")
		if action == "" {
			return
		}
		addGUIToken(out, action)
		for _, screen := range []string{
			firstText(typed, "screen_id", "screen", "screen_name"),
			firstText(typed, "path", "route", "url"),
		} {
			if screen != "" {
				addGUIToken(out, screen+":"+action)
				addGUIToken(out, screen+"."+action)
			}
		}
	}
}

func guiActionStatusCounts(item map[string]any) bool {
	status := strings.ToLower(firstText(item, "status", "result"))
	switch status {
	case "", "passed", "ok", "clicked", "selected", "opened", "visible":
		return true
	default:
		return false
	}
}

func guiScreenVisited(screen journey.GUIScreenMatrixRow, visited map[string]bool) bool {
	for _, token := range []string{screen.ID, screen.Path} {
		if visited[normalizeGUIToken(token)] {
			return true
		}
	}
	return false
}

func guiActionCompleted(screen journey.GUIScreenMatrixRow, action string, actions map[string]bool) bool {
	action = normalizeGUIToken(action)
	if actions[action] {
		return true
	}
	for _, screenToken := range []string{screen.ID, screen.Path} {
		screenToken = normalizeGUIToken(screenToken)
		if screenToken == "" {
			continue
		}
		if actions[screenToken+":"+action] || actions[screenToken+"."+action] {
			return true
		}
	}
	return false
}

func guiScreenLabel(screen journey.GUIScreenMatrixRow) string {
	if label := strings.TrimSpace(screen.ID); label != "" {
		return label
	}
	if label := strings.TrimSpace(screen.Path); label != "" {
		return label
	}
	return "screen"
}

func addGUIToken(out map[string]bool, value string) {
	if token := normalizeGUIToken(value); token != "" {
		out[token] = true
	}
}

func normalizeGUIToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
