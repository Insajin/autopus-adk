//go:build !darwin && !linux

package promptlayer

import "errors"

func readOMPContextPromotionRuntimeBundleV3(string) (
	[]byte, []byte, []byte, []byte, string, error,
) {
	return nil, nil, nil, nil, "", errors.New("OMP context promotion runtime v3 loading is unsupported")
}
