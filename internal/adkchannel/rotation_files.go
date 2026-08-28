package adkchannel

import (
	"fmt"
	"strings"
)

const (
	maxTagPublicKeyPinBytes = 256
	maxFingerprintPinBytes  = 128
	maxPromotionKeyPinBytes = 128
)

func VerifyRotationFiles(
	documentPath string,
	signaturePath string,
	pinFiles RotationPinFiles,
	options RotationOptions,
) (RotationVerified, error) {
	return verifyRotationFiles(documentPath, signaturePath, pinFiles, options, false)
}

func VerifyRotationFilesHistorical(
	documentPath string,
	signaturePath string,
	pinFiles RotationPinFiles,
	options RotationOptions,
) (RotationVerified, error) {
	return verifyRotationFiles(documentPath, signaturePath, pinFiles, options, true)
}

func verifyRotationFiles(
	documentPath string,
	signaturePath string,
	pinFiles RotationPinFiles,
	options RotationOptions,
	historical bool,
) (RotationVerified, error) {
	signed, err := ReadSignedFiles(documentPath, signaturePath)
	if err != nil {
		return RotationVerified{}, err
	}
	pins, err := readRotationPins(pinFiles)
	if err != nil {
		return RotationVerified{}, err
	}
	if historical {
		return VerifyRotationHistorical(signed, pins, options)
	}
	return VerifyRotation(signed, pins, options)
}

func readRotationPins(files RotationPinFiles) (RotationPins, error) {
	tagPublicKey, err := readCanonicalPin(files.NextTagPublicKeyPath, maxTagPublicKeyPinBytes)
	if err != nil {
		return RotationPins{}, fmt.Errorf("read next tag public key pin: %w", err)
	}
	tagFingerprint, err := readCanonicalPin(files.NextTagFingerprintPath, maxFingerprintPinBytes)
	if err != nil {
		return RotationPins{}, fmt.Errorf("read next tag fingerprint pin: %w", err)
	}
	promotionPublicKey, err := readCanonicalPin(files.NextPromotionPublicKeyPath, maxPromotionKeyPinBytes)
	if err != nil {
		return RotationPins{}, fmt.Errorf("read next promotion public key pin: %w", err)
	}
	return RotationPins{
		NextTagPublicKey:       tagPublicKey,
		NextTagFingerprint:     tagFingerprint,
		NextPromotionPublicKey: promotionPublicKey,
	}, nil
}

func readCanonicalPin(path string, maximum int) (string, error) {
	encoded, err := readRegularFile(path, maximum)
	if err != nil {
		return "", err
	}
	if len(encoded) < 2 || encoded[len(encoded)-1] != '\n' {
		return "", fmt.Errorf("pin file is not a canonical LF-terminated line")
	}
	value := string(encoded[:len(encoded)-1])
	if strings.ContainsAny(value, "\r\n\x00") {
		return "", fmt.Errorf("pin file is not a canonical LF-terminated line")
	}
	return value, nil
}
