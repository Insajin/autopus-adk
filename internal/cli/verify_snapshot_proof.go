package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	snapshotProofEntryName = "autopus/snapshot-comparison-proof.json"
	snapshotProofMaxBytes  = 64 << 10
)

type snapshotComparisonProof struct {
	Version           int
	Nonce             string
	PlaywrightVersion string
	UpdateSnapshots   string
	Projects          []snapshotProjectProof
	RequiredProjects  []string
	Diagnostic        string
	Legacy            bool
}

type snapshotProjectProof struct {
	Name               string
	ComparisonsEnabled bool
	IgnoreSnapshots    *bool
	State              string
	Source             string
	Dependencies       []string
	Teardown           string
	SupportOnly        bool
}

type snapshotProofV2 struct {
	Version           int                      `json:"version"`
	Nonce             string                   `json:"nonce,omitempty"`
	PlaywrightVersion string                   `json:"playwright_version"`
	UpdateSnapshots   string                   `json:"update_snapshots"`
	Projects          []snapshotProjectProofV2 `json:"projects"`
	RequiredProjects  []string                 `json:"required_projects,omitempty"`
	Diagnostic        string                   `json:"diagnostic,omitempty"`
}

type snapshotProjectProofV2 struct {
	Name            string   `json:"name"`
	IgnoreSnapshots *bool    `json:"ignore_snapshots"`
	State           string   `json:"state"`
	Source          string   `json:"source"`
	Dependencies    []string `json:"dependencies,omitempty"`
	Teardown        string   `json:"teardown,omitempty"`
	SupportOnly     bool     `json:"support_only,omitempty"`
}

func readSnapshotComparisonProof(path, nonce string) (snapshotComparisonProof, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return snapshotComparisonProof{}, fmt.Errorf("snapshot comparison proof 누락: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > snapshotProofMaxBytes {
		return snapshotComparisonProof{}, fmt.Errorf("snapshot comparison proof 파일이 안전하지 않습니다")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return snapshotComparisonProof{}, fmt.Errorf("snapshot comparison proof 읽기 실패: %w", err)
	}
	proof, err := decodeSnapshotComparisonProof(raw)
	if err != nil {
		return snapshotComparisonProof{}, err
	}
	if nonce == "" || proof.Nonce != nonce {
		return snapshotComparisonProof{}, fmt.Errorf("snapshot comparison proof nonce가 일치하지 않습니다")
	}
	return proof, nil
}

func decodeSnapshotComparisonProof(raw []byte) (snapshotComparisonProof, error) {
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return snapshotComparisonProof{}, fmt.Errorf("snapshot comparison proof 형식 오류: %w", err)
	}
	if header.Version == 1 {
		return decodeLegacySnapshotProof(raw)
	}
	if header.Version != 2 {
		return snapshotComparisonProof{}, fmt.Errorf("snapshot comparison proof 계약이 불완전합니다")
	}
	var wire snapshotProofV2
	if err := decodeStrictJSON(raw, &wire); err != nil {
		return snapshotComparisonProof{}, fmt.Errorf("snapshot comparison proof 형식 오류: %w", err)
	}
	if err := validateSnapshotProofV2(wire); err != nil {
		return snapshotComparisonProof{}, err
	}
	proof := snapshotComparisonProof{
		Version: 2, Nonce: wire.Nonce, PlaywrightVersion: wire.PlaywrightVersion,
		UpdateSnapshots: wire.UpdateSnapshots, RequiredProjects: wire.RequiredProjects, Diagnostic: wire.Diagnostic,
	}
	for _, project := range wire.Projects {
		proof.Projects = append(proof.Projects, snapshotProjectProof{
			Name: project.Name, IgnoreSnapshots: project.IgnoreSnapshots, State: project.State,
			Source: project.Source, Dependencies: project.Dependencies, Teardown: project.Teardown,
			SupportOnly: project.SupportOnly, ComparisonsEnabled: project.State == "enabled",
		})
	}
	return proof, nil
}

func decodeLegacySnapshotProof(raw []byte) (snapshotComparisonProof, error) {
	var legacy struct {
		Version  int    `json:"version"`
		Nonce    string `json:"nonce,omitempty"`
		Projects []struct {
			Name               string `json:"name"`
			ComparisonsEnabled bool   `json:"comparisons_enabled"`
		} `json:"projects"`
	}
	if err := decodeStrictJSON(raw, &legacy); err != nil {
		return snapshotComparisonProof{}, fmt.Errorf("snapshot comparison proof 형식 오류: %w", err)
	}
	if len(legacy.Projects) == 0 {
		return snapshotComparisonProof{}, fmt.Errorf("snapshot comparison proof 계약이 불완전합니다")
	}
	proof := snapshotComparisonProof{Version: 1, Nonce: legacy.Nonce, Legacy: true, Diagnostic: "legacy proof cannot establish public snapshot policy"}
	seen := map[string]struct{}{}
	for _, project := range legacy.Projects {
		if strings.TrimSpace(project.Name) == "" {
			return snapshotComparisonProof{}, fmt.Errorf("snapshot comparison proof 프로젝트 이름이 비어 있습니다")
		}
		if _, duplicate := seen[project.Name]; duplicate {
			return snapshotComparisonProof{}, fmt.Errorf("snapshot comparison proof 프로젝트가 중복되었습니다")
		}
		seen[project.Name] = struct{}{}
		proof.Projects = append(proof.Projects, snapshotProjectProof{
			Name: project.Name, ComparisonsEnabled: project.ComparisonsEnabled,
			State: "unproven", Source: "unsupported",
		})
	}
	return proof, nil
}

func decodeStrictJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("뒤에 추가 데이터가 있습니다")
	}
	return nil
}

func validateSnapshotProofV2(proof snapshotProofV2) error {
	allowedUpdates := map[string]bool{"none": true, "missing": true, "changed": true, "all": true}
	if proof.Version != 2 || proof.Nonce == "" || proof.PlaywrightVersion == "" || !allowedUpdates[proof.UpdateSnapshots] || len(proof.Projects) == 0 {
		return fmt.Errorf("snapshot comparison proof 계약이 불완전합니다")
	}
	seen := map[string]struct{}{}
	for _, project := range proof.Projects {
		if strings.TrimSpace(project.Name) == "" || strings.TrimSpace(project.Name) != project.Name {
			return fmt.Errorf("snapshot comparison proof 프로젝트 이름이 올바르지 않습니다")
		}
		if _, duplicate := seen[project.Name]; duplicate {
			return fmt.Errorf("snapshot comparison proof 프로젝트가 중복되었습니다")
		}
		seen[project.Name] = struct{}{}
		if err := validateSnapshotProjectV2(project, proof.UpdateSnapshots); err != nil {
			return fmt.Errorf("snapshot comparison proof 프로젝트 %q: %w", project.Name, err)
		}
	}
	requiredSeen := map[string]struct{}{}
	for _, required := range proof.RequiredProjects {
		if strings.TrimSpace(required) == "" || strings.TrimSpace(required) != required {
			return fmt.Errorf("snapshot comparison proof required project 이름이 올바르지 않습니다")
		}
		if _, duplicate := requiredSeen[required]; duplicate {
			return fmt.Errorf("snapshot comparison proof required project가 중복되었습니다")
		}
		requiredSeen[required] = struct{}{}
	}
	return nil
}

func validateSnapshotProjectV2(project snapshotProjectProofV2, updateSnapshots string) error {
	if project.Source != "public" && project.Source != "unsupported" && project.Source != "missing" {
		return fmt.Errorf("source가 올바르지 않습니다")
	}
	if project.Source == "public" && project.IgnoreSnapshots == nil || project.Source != "public" && project.IgnoreSnapshots != nil {
		return fmt.Errorf("ignore_snapshots와 source가 일치하지 않습니다")
	}
	wantState := "unproven"
	if project.Source == "public" && *project.IgnoreSnapshots {
		wantState = "disabled"
	} else if project.Source == "public" && !*project.IgnoreSnapshots && updateSnapshots == "none" {
		wantState = "enabled"
	}
	if project.State != wantState {
		return fmt.Errorf("state가 resolved policy와 일치하지 않습니다")
	}
	if project.Teardown != "" && strings.TrimSpace(project.Teardown) != project.Teardown {
		return fmt.Errorf("teardown 이름이 올바르지 않습니다")
	}
	seen := map[string]struct{}{}
	for _, dependency := range project.Dependencies {
		if strings.TrimSpace(dependency) == "" || strings.TrimSpace(dependency) != dependency {
			return fmt.Errorf("dependency 이름이 올바르지 않습니다")
		}
		if _, duplicate := seen[dependency]; duplicate {
			return fmt.Errorf("dependency가 중복되었습니다")
		}
		seen[dependency] = struct{}{}
	}
	return nil
}
