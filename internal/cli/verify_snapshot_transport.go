package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

func (proof snapshotComparisonProof) MarshalJSON() ([]byte, error) {
	if proof.Version == 1 {
		projects := make([]map[string]any, 0, len(proof.Projects))
		for _, project := range proof.Projects {
			projects = append(projects, map[string]any{"name": project.Name, "comparisons_enabled": project.ComparisonsEnabled})
		}
		return json.Marshal(struct {
			Version  int              `json:"version"`
			Nonce    string           `json:"nonce,omitempty"`
			Projects []map[string]any `json:"projects"`
		}{proof.Version, proof.Nonce, projects})
	}
	wire := snapshotProofV2{
		Version: 2, Nonce: proof.Nonce, PlaywrightVersion: proof.PlaywrightVersion,
		UpdateSnapshots: proof.UpdateSnapshots, RequiredProjects: proof.RequiredProjects, Diagnostic: proof.Diagnostic,
	}
	for _, project := range proof.Projects {
		wire.Projects = append(wire.Projects, snapshotProjectProofV2{
			Name: project.Name, IgnoreSnapshots: project.IgnoreSnapshots, State: project.State,
			Source: project.Source, Dependencies: project.Dependencies, Teardown: project.Teardown,
			SupportOnly: project.SupportOnly,
		})
	}
	return json.Marshal(wire)
}

func appendSnapshotProofToBlob(blob []byte, proof snapshotComparisonProof) ([]byte, error) {
	return appendSnapshotProofToBlobWithLimit(blob, proof, maxBlobArchiveBytes)
}

func appendSnapshotProofToBlobWithLimit(blob []byte, proof snapshotComparisonProof, limit int) ([]byte, error) {
	if limit < 0 || len(blob) > limit {
		return nil, fmt.Errorf("snapshot proof blob이 %d바이트 제한을 초과했습니다", limit)
	}
	reader, err := zip.NewReader(bytes.NewReader(blob), int64(len(blob)))
	if err != nil {
		return nil, fmt.Errorf("snapshot proof blob 열기 실패: %w", err)
	}
	proofRaw, err := json.Marshal(proof)
	if err != nil {
		return nil, fmt.Errorf("snapshot proof 인코딩 실패: %w", err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range reader.File {
		if file.Name == snapshotProofEntryName {
			_ = writer.Close()
			return nil, fmt.Errorf("blob에 예약된 snapshot proof entry가 이미 있습니다")
		}
		if err := writer.Copy(file); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("snapshot proof blob 복사 실패: %w", err)
		}
	}
	entry, err := writer.CreateHeader(&zip.FileHeader{Name: snapshotProofEntryName, Method: zip.Deflate})
	if err == nil {
		_, err = entry.Write(proofRaw)
	}
	if closeErr := writer.Close(); closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	if err != nil {
		return nil, fmt.Errorf("snapshot proof blob 기록 실패: %w", err)
	}
	if output.Len() > limit {
		return nil, fmt.Errorf("snapshot proof가 기록된 blob이 %d바이트 제한을 초과했습니다", limit)
	}
	return output.Bytes(), nil
}

func annotateSnapshotProofJSON(report []byte, proof snapshotComparisonProof) ([]byte, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(report, &object); err != nil {
		return nil, fmt.Errorf("snapshot proof JSON report 해석 실패: %w", err)
	}
	if object == nil {
		return nil, fmt.Errorf("snapshot proof JSON report가 객체가 아닙니다")
	}
	proofRaw, err := json.Marshal(proof)
	if err != nil {
		return nil, err
	}
	object["_autopusSnapshotComparisonProof"] = proofRaw
	return json.Marshal(object)
}
