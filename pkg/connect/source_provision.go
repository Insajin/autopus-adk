package connect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// KnowledgeSource represents a knowledge source returned by the Platform.
type KnowledgeSource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// createSourceRequest is the payload for POST /knowledge/sources.
type createSourceRequest struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// bridgeBindingRequest is the payload for PUT /bridge/binding.
type bridgeBindingRequest struct {
	ManifestVersion string `json:"manifest_version"`
	WorkspaceRoot   string `json:"workspace_root"`
	SyncMode        string `json:"sync_mode"`
}

// ProvisionBridgeSource creates a bridge_sync knowledge source and binding
// for the ADK Worker. Returns the source ID. If a source already exists
// for this device, the backend should return the existing one.
func (c *Client) ProvisionBridgeSource(ctx context.Context, workspaceID, workDir string) (string, error) {
	sourceID, err := c.createKnowledgeSource(ctx, workspaceID)
	if err != nil {
		return "", fmt.Errorf("provision bridge source: create: %w", err)
	}

	if err := c.upsertBridgeBinding(ctx, workspaceID, sourceID, workDir); err != nil {
		return "", fmt.Errorf("provision bridge source: binding: %w", err)
	}

	return sourceID, nil
}

func (c *Client) createKnowledgeSource(ctx context.Context, workspaceID string) (string, error) {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	payload := createSourceRequest{
		Type: "bridge_sync",
		Name: fmt.Sprintf("ADK Worker - %s", hostname),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/api/v1/workspaces/%s/knowledge/sources",
		c.serverURL, workspaceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("create source failed (%d): %s", resp.StatusCode, respBody)
	}

	result, err := unwrapJSON[KnowledgeSource](respBody)
	if err != nil {
		return "", err
	}
	return result.ID, nil
}

func (c *Client) upsertBridgeBinding(ctx context.Context, workspaceID, sourceID, workDir string) error {
	payload := bridgeBindingRequest{
		ManifestVersion: "1.0",
		WorkspaceRoot:   workDir,
		SyncMode:        "push",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/workspaces/%s/knowledge/sources/%s/bridge/binding",
		c.serverURL, workspaceID, sourceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upsert binding failed (%d): %s", resp.StatusCode, respBody)
	}
	return nil
}
