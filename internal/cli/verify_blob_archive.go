package cli

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type safeBlobReport struct {
	Report          []byte
	Entries         map[string]struct{}
	SnapshotProof   snapshotComparisonProof
	ProofPresent    bool
	ProofDiagnostic string
}

func readSafeBlobReport(output []byte) (safeBlobReport, bool) {
	archive, err := readSafeBlobReportDetailed(output)
	return archive, err == nil
}

func readSafeBlobReportDetailed(output []byte) (safeBlobReport, error) {
	reader, err := zip.NewReader(bytes.NewReader(output), int64(len(output)))
	if err != nil {
		return safeBlobReport{}, fmt.Errorf("blob archive 형식이 올바르지 않습니다")
	}
	if len(reader.File) > maxBlobEntries {
		return safeBlobReport{}, fmt.Errorf("blob entry 제한을 초과했습니다")
	}
	entries := make(map[string]struct{}, len(reader.File))
	var reportFile, proofFile *zip.File
	var total uint64
	for _, file := range reader.File {
		if !safeBlobEntryName(file.Name) || file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return safeBlobReport{}, fmt.Errorf("blob entry 경로가 안전하지 않습니다")
		}
		if _, duplicate := entries[file.Name]; duplicate {
			return safeBlobReport{}, fmt.Errorf("blob entry가 중복되었습니다")
		}
		if file.UncompressedSize64 > maxBlobEntryBytes || total > maxBlobTotalBytes-file.UncompressedSize64 {
			return safeBlobReport{}, fmt.Errorf("blob uncompressed 크기 제한을 초과했습니다")
		}
		if file.UncompressedSize64 > 0 && file.CompressedSize64 == 0 {
			return safeBlobReport{}, fmt.Errorf("blob entry 압축 메타데이터가 올바르지 않습니다")
		}
		if file.UncompressedSize64 > 1<<20 && file.CompressedSize64 > 0 && file.UncompressedSize64/file.CompressedSize64 > 1_000 {
			return safeBlobReport{}, fmt.Errorf("blob entry 압축 비율 제한을 초과했습니다")
		}
		total += file.UncompressedSize64
		entries[file.Name] = struct{}{}
		switch file.Name {
		case "report.jsonl":
			reportFile = file
		case snapshotProofEntryName:
			proofFile = file
		}
	}
	if reportFile == nil {
		return safeBlobReport{}, fmt.Errorf("blob report.jsonl이 누락되었습니다")
	}
	report, ok := readLimitedBlobEntry(reportFile, maxBlobEntryBytes)
	if !ok {
		return safeBlobReport{}, fmt.Errorf("blob report 읽기 제한을 초과했습니다")
	}
	archive := safeBlobReport{Report: report, Entries: entries}
	if proofFile == nil {
		return archive, nil
	}
	raw, ok := readLimitedBlobEntry(proofFile, snapshotProofMaxBytes)
	if !ok {
		return safeBlobReport{}, fmt.Errorf("blob snapshot proof 읽기 제한을 초과했습니다")
	}
	proof, err := decodeSnapshotComparisonProof(raw)
	if err != nil {
		archive.ProofDiagnostic = "snapshot comparison proof is invalid"
		return archive, nil
	}
	archive.SnapshotProof = proof
	archive.ProofPresent = true
	return archive, nil
}

func readLimitedBlobEntry(file *zip.File, limit int64) ([]byte, bool) {
	stream, err := file.Open()
	if err != nil {
		return nil, false
	}
	defer func() { _ = stream.Close() }()
	raw, err := io.ReadAll(io.LimitReader(stream, limit+1))
	return raw, err == nil && int64(len(raw)) <= limit
}

func safeBlobEntryName(name string) bool {
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || filepath.VolumeName(name) != "" {
		return false
	}
	cleaned := path.Clean(strings.TrimSuffix(name, "/"))
	return cleaned != "." && cleaned == strings.TrimSuffix(name, "/") && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}
