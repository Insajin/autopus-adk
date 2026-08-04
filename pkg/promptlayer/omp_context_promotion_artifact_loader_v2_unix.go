//go:build darwin || linux

package promptlayer

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type ompContextPromotionArtifactDirectoryV2 struct {
	name string
	fd   int
	stat unix.Stat_t
}

type ompContextPromotionArtifactEntryV2 struct {
	name  string
	file  *os.File
	stat  unix.Stat_t
	limit int64
}

type ompContextPromotionArtifactTransactionV2 struct {
	rootPath    string
	directories []ompContextPromotionArtifactDirectoryV2
	report      ompContextPromotionArtifactEntryV2
	attestation ompContextPromotionArtifactEntryV2
}

func readOMPContextPromotionArtifactPairV2WithHook(
	root string,
	hook ompContextPromotionArtifactReadHookV2,
) (reportBytes, attestationBytes []byte, err error) {
	transaction, err := openOMPContextPromotionArtifactTransactionV2(root, hook)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if closeErr := transaction.close(); closeErr != nil {
			reportBytes = nil
			attestationBytes = nil
			err = errors.Join(err, closeErr)
		}
	}()
	reportBytes, err = transaction.readEntry(&transaction.report)
	if err != nil {
		return nil, nil, err
	}
	attestationBytes, err = transaction.readEntry(&transaction.attestation)
	if err != nil {
		return nil, nil, err
	}
	if err := transaction.recheck(); err != nil {
		return nil, nil, err
	}
	reportAgain, err := transaction.readEntry(&transaction.report)
	if err != nil || !bytes.Equal(reportBytes, reportAgain) {
		return nil, nil, fmt.Errorf("OMP context promotion report changed while reading")
	}
	attestationAgain, err := transaction.readEntry(&transaction.attestation)
	if err != nil || !bytes.Equal(attestationBytes, attestationAgain) {
		return nil, nil, fmt.Errorf("OMP context promotion attestation changed while reading")
	}
	if err := transaction.recheck(); err != nil {
		return nil, nil, err
	}
	return reportBytes, attestationBytes, nil
}

func openOMPContextPromotionArtifactTransactionV2(
	root string,
	hook ompContextPromotionArtifactReadHookV2,
) (*ompContextPromotionArtifactTransactionV2, error) {
	if root == "" || filepath.Clean(root) == "." || filepath.Clean(root) == string(filepath.Separator) {
		return nil, fmt.Errorf("OMP context promotion root is required")
	}
	absolute, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, fmt.Errorf("resolve OMP context promotion root")
	}
	rootInfo, err := os.Lstat(absolute)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("OMP context promotion root must be a regular directory")
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil || resolved != absolute {
		return nil, fmt.Errorf("OMP context promotion root identity is unsafe")
	}
	rootFD, rootStat, err := openOMPContextPromotionCanonicalDirectoryV2(resolved)
	if err != nil || !secureOMPContextPromotionRootStatV2(rootStat) {
		if rootFD >= 0 {
			_ = unix.Close(rootFD)
		}
		return nil, fmt.Errorf("OMP context promotion root authority is unsafe")
	}
	transaction := &ompContextPromotionArtifactTransactionV2{
		rootPath: resolved,
		directories: []ompContextPromotionArtifactDirectoryV2{{
			fd: rootFD, stat: rootStat,
		}},
	}
	openDirectory := func(name, hookStage string, validator func(unix.Stat_t) bool) error {
		if err := runOMPContextPromotionArtifactReadHookV2(hook, hookStage); err != nil {
			return err
		}
		parent := transaction.directories[len(transaction.directories)-1]
		directory, err := openOMPContextPromotionChildDirectoryV2(parent.fd, name, validator)
		if err != nil {
			return err
		}
		transaction.directories = append(transaction.directories, directory)
		return nil
	}
	if err := openDirectory(".autopus", "before_autopus_open", secureOMPContextPromotionMetadataDirectoryStatV2); err != nil {
		_ = transaction.close()
		return nil, err
	}
	if err := openDirectory("runtime", "before_runtime_open", secureOMPContextPromotionPrivateDirectoryStatV2); err != nil {
		_ = transaction.close()
		return nil, err
	}
	if err := openDirectory("omp-context", "before_artifact_directory_open", secureOMPContextPromotionPrivateDirectoryStatV2); err != nil {
		_ = transaction.close()
		return nil, err
	}
	artifactFD := transaction.directories[len(transaction.directories)-1].fd
	if err := runOMPContextPromotionArtifactReadHookV2(hook, "before_report_open"); err != nil {
		_ = transaction.close()
		return nil, err
	}
	transaction.report, err = openOMPContextPromotionArtifactEntryV2(
		artifactFD, ompContextPromotionReportFileNameV2, ompContextPromotionReportMaxBytesV1,
	)
	if err != nil {
		_ = transaction.close()
		return nil, err
	}
	if err := runOMPContextPromotionArtifactReadHookV2(hook, "before_attestation_open"); err != nil {
		_ = transaction.close()
		return nil, err
	}
	transaction.attestation, err = openOMPContextPromotionArtifactEntryV2(
		artifactFD, ompContextPromotionAttestationFileNameV2, ompContextPromotionAttestationMaxBytesV2,
	)
	if err != nil || sameOMPContextPromotionUnixIdentityV2(transaction.report.stat, transaction.attestation.stat) {
		_ = transaction.close()
		return nil, fmt.Errorf("OMP context promotion artifact pair is unsafe")
	}
	return transaction, nil
}
