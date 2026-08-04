//go:build !darwin && !linux

package promptlayer

import "fmt"

func readOMPContextPromotionArtifactPairV2WithHook(
	string,
	ompContextPromotionArtifactReadHookV2,
) ([]byte, []byte, error) {
	return nil, nil, fmt.Errorf("OMP context promotion artifact loading is unsupported on this platform")
}
