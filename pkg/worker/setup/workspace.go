package setup

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Workspace represents a user's workspace on the Autopus platform.
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FetchWorkspaces retrieves the list of workspaces from the backend.
func FetchWorkspaces(backendURL, token string) ([]Workspace, error) {
	endpoint := strings.TrimRight(backendURL, "/") + "/api/v1/workspaces"

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch workspaces: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch workspaces failed (%d): %s", resp.StatusCode, body)
	}

	result, err := unwrap[[]Workspace](body)
	if err != nil {
		return nil, fmt.Errorf("decode workspaces: %w", err)
	}
	return *result, nil
}

// FindWorkspaceByID fetches all workspaces and returns the one matching id.
// Used by non-interactive setup (--workspace flag) to validate the ID and get the name.
func FindWorkspaceByID(backendURL, token, id string) (*Workspace, error) {
	workspaces, err := FetchWorkspaces(backendURL, token)
	if err != nil {
		return nil, err
	}
	for _, ws := range workspaces {
		if ws.ID == id {
			return &ws, nil
		}
	}
	return nil, fmt.Errorf("workspace %q not found", id)
}

// SelectWorkspace picks the workspace to use. Auto-selects if only one is available.
// For multiple workspaces, prompts the user to select.
func SelectWorkspace(workspaces []Workspace) (*Workspace, error) {
	if len(workspaces) == 0 {
		return nil, fmt.Errorf("no workspaces available")
	}
	if len(workspaces) == 1 {
		return &workspaces[0], nil
	}

	fmt.Println("Available workspaces:")
	for i, ws := range workspaces {
		fmt.Printf("  [%d] %s (ID: %s)\n", i+1, ws.Name, ws.ID)
	}

	var choice int
	for {
		fmt.Print("Select workspace (1-", len(workspaces), "): ")
		if _, err := fmt.Scan(&choice); err != nil {
			fmt.Println("Invalid input, please enter a number.")
			continue
		}
		if choice >= 1 && choice <= len(workspaces) {
			return &workspaces[choice-1], nil
		}
		fmt.Printf("Please enter a number between 1 and %d.\n", len(workspaces))
	}
}
