//go:build !darwin && !linux

package promptlayer

import "errors"

func currentOMPContextPromotionExecutableSHA256V3() (string, error) {
	return "", errors.New("OMP context promotion runtime executable hashing is unsupported")
}
