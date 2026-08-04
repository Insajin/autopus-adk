//go:build darwin || linux

package promptlayer

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
)

func readOMPContextPromotionRuntimeBundleV3(root string) (
	reportBytes, attestationBytes, lineageBytes, lineageSignature []byte,
	releaseKeyBundle string,
	returnErr error,
) {
	transaction, err := openOMPContextPromotionArtifactTransactionV2(root, nil)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}
	artifactFD := transaction.directories[len(transaction.directories)-1].fd
	lineage, err := openOMPContextPromotionArtifactEntryV2(
		artifactFD, ompContextPromotionReleaseLineageFileNameV3,
		ompContextPromotionReleaseLineageMaxBytesV3,
	)
	if err != nil {
		_ = transaction.close()
		return nil, nil, nil, nil, "", err
	}
	lineageSignatureEntry, err := openOMPContextPromotionArtifactEntryV2(
		artifactFD, ompContextPromotionReleaseLineageSignatureV3,
		ompContextPromotionReleaseLineageSignatureSizeV3,
	)
	if err != nil {
		_ = lineage.file.Close()
		_ = transaction.close()
		return nil, nil, nil, nil, "", err
	}
	defer func() {
		closeErr := errors.Join(lineage.file.Close(), lineageSignatureEntry.file.Close(), transaction.close())
		if closeErr != nil {
			reportBytes, attestationBytes, lineageBytes, lineageSignature = nil, nil, nil, nil
			releaseKeyBundle = ""
			returnErr = errors.Join(returnErr, fmt.Errorf("close OMP context promotion runtime v3 bundle: %w", closeErr))
		}
	}()
	identities := []ompContextPromotionArtifactEntryV2{
		transaction.report, transaction.attestation, lineage, lineageSignatureEntry,
	}
	for left := range identities {
		for right := left + 1; right < len(identities); right++ {
			if sameOMPContextPromotionUnixIdentityV2(identities[left].stat, identities[right].stat) {
				return nil, nil, nil, nil, "", errors.New("OMP context promotion runtime v3 bundle identities overlap")
			}
		}
	}
	readAll := func() ([][]byte, error) {
		entries := []*ompContextPromotionArtifactEntryV2{
			&transaction.report, &transaction.attestation, &lineage, &lineageSignatureEntry,
		}
		bodies := make([][]byte, 0, len(entries))
		for _, entry := range entries {
			body, readErr := transaction.readEntry(entry)
			if readErr != nil {
				return nil, readErr
			}
			bodies = append(bodies, body)
		}
		return bodies, nil
	}
	first, err := readAll()
	if err != nil || transaction.recheck() != nil {
		return nil, nil, nil, nil, "", errors.New("OMP context promotion runtime v3 bundle is unavailable")
	}
	second, err := readAll()
	if err != nil || transaction.recheck() != nil {
		return nil, nil, nil, nil, "", errors.New("OMP context promotion runtime v3 bundle changed")
	}
	for index := range first {
		if !bytes.Equal(first[index], second[index]) {
			return nil, nil, nil, nil, "", errors.New("OMP context promotion runtime v3 bundle changed")
		}
	}
	if len(first[3]) != ompContextPromotionReleaseLineageSignatureSizeV3 {
		return nil, nil, nil, nil, "", errors.New("OMP context promotion runtime v3 signature is invalid")
	}
	releaseKeyBundle = filepath.Join(
		transaction.rootPath, ".autopus", "runtime", "omp-context",
		ompContextPromotionReleaseKeyBundleDirectoryV3,
	)
	return first[0], first[1], first[2], first[3], releaseKeyBundle, nil
}
