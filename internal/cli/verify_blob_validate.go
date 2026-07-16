package cli

import "fmt"

func newBlobCollector(archive safeBlobReport) *blobCollector {
	return &blobCollector{
		tests:           map[string]blobTestInfo{},
		results:         map[string]*blobResult{},
		stepsByResult:   map[string][]*blobStep{},
		stepsByID:       map[string]map[string]*blobStep{},
		entries:         archive.Entries,
		projectSet:      map[string]struct{}{},
		snapshotProof:   archive.SnapshotProof,
		proofPresent:    archive.ProofPresent,
		proofDiagnostic: archive.ProofDiagnostic,
	}
}

func validatePlaywrightBlob(output []byte) error {
	if len(output) > maxBlobArchiveBytes {
		return fmt.Errorf("blob archive가 %d바이트 제한을 초과했습니다", maxBlobArchiveBytes)
	}
	archive, err := readSafeBlobReportDetailed(output)
	if err != nil {
		return err
	}
	if !newBlobCollector(archive).collect(archive.Report) {
		return fmt.Errorf("blob report event 또는 line 제한을 위반했습니다")
	}
	return nil
}
