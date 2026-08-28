package adkchannel

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

func VerifyRotation(signed SignedBytes, pins RotationPins, options RotationOptions) (RotationVerified, error) {
	return verifyRotation(signed, pins, options, true)
}

func VerifyRotationHistorical(signed SignedBytes, pins RotationPins, options RotationOptions) (RotationVerified, error) {
	return verifyRotation(signed, pins, options, false)
}

func verifyRotation(
	signed SignedBytes,
	pins RotationPins,
	options RotationOptions,
	enforceCurrentWindow bool,
) (RotationVerified, error) {
	if len(signed.Document) == 0 || len(signed.Document) > MaxDocumentBytes {
		return RotationVerified{}, fmt.Errorf("document size must be between 1 and %d bytes", MaxDocumentBytes)
	}
	if len(signed.Signature) != ed25519.SignatureSize {
		return RotationVerified{}, fmt.Errorf("signature must be %d bytes", ed25519.SignatureSize)
	}
	if err := validateRotationExpectations(pins, options, enforceCurrentWindow); err != nil {
		return RotationVerified{}, err
	}

	document, err := parseRotationDocument(signed.Document)
	if err != nil {
		return RotationVerified{}, err
	}
	publicKey, err := decodeCanonicalBase64(options.PublicKeyBase64, ed25519.PublicKeySize)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return RotationVerified{}, fmt.Errorf("channel public key must be canonical base64 for %d bytes", ed25519.PublicKeySize)
	}
	message := make([]byte, len(rotationSignatureDomain)+len(signed.Document))
	copy(message, rotationSignatureDomain)
	copy(message[len(rotationSignatureDomain):], signed.Document)
	if !ed25519.Verify(ed25519.PublicKey(publicKey), message, signed.Signature) {
		return RotationVerified{}, fmt.Errorf("rotation signature verification failed")
	}
	if err := validateRotationDocument(document, pins, options, enforceCurrentWindow); err != nil {
		return RotationVerified{}, err
	}
	return RotationVerified{
		Document:          document,
		CanonicalDocument: append([]byte(nil), signed.Document...),
	}, nil
}

func validateRotationExpectations(pins RotationPins, options RotationOptions, requireClock bool) error {
	if !validKeyID(options.ExpectedKeyID) {
		return fmt.Errorf("expected channel key id is invalid")
	}
	if requireClock && options.Now == nil {
		return fmt.Errorf("clock is required")
	}
	if err := validateGitOID(options.ExpectedSourceCommit); err != nil {
		return fmt.Errorf("expected source commit is invalid: %w", err)
	}
	if err := validateGitOID(options.ExpectedSourceTree); err != nil {
		return fmt.Errorf("expected source tree is invalid: %w", err)
	}
	if pins.NextTagPublicKey != rotationNextTagPublicKey || pins.NextTagFingerprint != rotationNextTagFingerprint {
		return fmt.Errorf("next tag signer pins do not match the authorized rotation")
	}
	if pins.NextPromotionPublicKey != rotationNextPromotionPublicKey {
		return fmt.Errorf("next promotion public key pin does not match the authorized rotation")
	}
	if err := validateTagSigner(pins.NextTagPublicKey, pins.NextTagFingerprint); err != nil {
		return fmt.Errorf("next tag signer pins are invalid: %w", err)
	}
	if err := validatePromotionKey(pins.NextPromotionPublicKey, rotationNextPromotionPublicKeySHA256); err != nil {
		return fmt.Errorf("next promotion public key pin is invalid: %w", err)
	}
	return nil
}

func validateRotationDocument(
	document RotationDocument,
	pins RotationPins,
	options RotationOptions,
	enforceCurrentWindow bool,
) error {
	expectations := [...]struct {
		field    string
		actual   string
		expected string
	}{
		{"schema_version", document.SchemaVersion, rotationSchema},
		{"channel", document.Channel, channelName},
		{"repository", document.Repository, channelRepository},
		{"bridge_tag", document.BridgeTag, rotationBridgeTag},
		{"release_mode", document.ReleaseMode, rotationReleaseMode},
		{"source_commit", document.SourceCommit, options.ExpectedSourceCommit},
		{"source_tree", document.SourceTree, options.ExpectedSourceTree},
		{"channel_key_id", document.ChannelKeyID, options.ExpectedKeyID},
	}
	for _, expectation := range expectations {
		if expectation.actual != expectation.expected {
			return fmt.Errorf("%s does not match the authorized value", expectation.field)
		}
	}
	if err := validateGitOID(document.SourceCommit); err != nil {
		return fmt.Errorf("source_commit is invalid: %w", err)
	}
	if err := validateGitOID(document.SourceTree); err != nil {
		return fmt.Errorf("source_tree is invalid: %w", err)
	}
	if document.PreviousTagFingerprint != rotationPreviousTagFingerprint {
		return fmt.Errorf("previous_tag_fingerprint does not match the authorized predecessor")
	}
	if err := validateFingerprint(document.PreviousTagFingerprint); err != nil {
		return fmt.Errorf("previous_tag_fingerprint is invalid: %w", err)
	}
	if document.NextTagPublicKey != pins.NextTagPublicKey || document.NextTagFingerprint != pins.NextTagFingerprint {
		return fmt.Errorf("next tag signer does not match checked-in pins")
	}
	if err := validateTagSigner(document.NextTagPublicKey, document.NextTagFingerprint); err != nil {
		return fmt.Errorf("next tag signer is invalid: %w", err)
	}
	if document.NextPromotionKeyID != rotationNextPromotionKeyID {
		return fmt.Errorf("next_promotion_key_id does not match the authorized key")
	}
	if document.NextPromotionPublicKey != pins.NextPromotionPublicKey {
		return fmt.Errorf("next promotion public key does not match the checked-in pin")
	}
	if document.NextPromotionPublicKeySHA256 != rotationNextPromotionPublicKeySHA256 {
		return fmt.Errorf("next promotion public key SHA-256 does not match the authorized digest")
	}
	if err := validatePromotionKey(document.NextPromotionPublicKey, document.NextPromotionPublicKeySHA256); err != nil {
		return fmt.Errorf("next promotion public key is invalid: %w", err)
	}
	return validateRotationValidity(document, options, enforceCurrentWindow)
}

func validateRotationValidity(document RotationDocument, options RotationOptions, enforceCurrentWindow bool) error {
	issuedAt, err := parseCanonicalUTC(document.IssuedAt)
	if err != nil {
		return fmt.Errorf("issued_at is invalid: %w", err)
	}
	expiresAt, err := parseCanonicalUTC(document.ExpiresAt)
	if err != nil {
		return fmt.Errorf("expires_at is invalid: %w", err)
	}
	if !issuedAt.Before(expiresAt) {
		return fmt.Errorf("issued_at must precede expires_at")
	}
	if expiresAt.Sub(issuedAt) > rotationMaxValidity {
		return fmt.Errorf("validity window exceeds %s", rotationMaxValidity)
	}
	if !enforceCurrentWindow {
		return nil
	}
	now := options.Now().UTC()
	if now.Before(issuedAt) {
		return fmt.Errorf("rotation document is from the future")
	}
	if !now.Before(expiresAt) {
		return fmt.Errorf("rotation document is stale")
	}
	return nil
}

func validateGitOID(value string) error {
	if len(value) != 40 || !isLowerHex(value) {
		return fmt.Errorf("must be 40 lowercase hexadecimal characters")
	}
	return nil
}

func isLowerHex(value string) bool {
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validKeyID(value string) bool {
	if len(value) < 1 || len(value) > 128 || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	previousHyphen := false
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
		if character == '-' {
			if previousHyphen {
				return false
			}
			previousHyphen = true
			continue
		}
		previousHyphen = false
	}
	return true
}

func validateFingerprint(value string) error {
	if !strings.HasPrefix(value, "SHA256:") {
		return fmt.Errorf("must use the SHA256: prefix")
	}
	decoded, err := base64.RawStdEncoding.Strict().DecodeString(strings.TrimPrefix(value, "SHA256:"))
	if err != nil || len(decoded) != sha256.Size || "SHA256:"+base64.RawStdEncoding.EncodeToString(decoded) != value {
		return fmt.Errorf("must contain a canonical unpadded SHA-256 digest")
	}
	return nil
}

func validateTagSigner(publicText, fingerprint string) error {
	parts := strings.Split(publicText, " ")
	if len(parts) != 2 || parts[0] != ssh.KeyAlgoED25519 {
		return fmt.Errorf("public key must be canonical ssh-ed25519")
	}
	blob, err := decodeCanonicalBase64(parts[1], 256)
	if err != nil {
		return fmt.Errorf("public key blob is not canonical base64")
	}
	publicKey, err := ssh.ParsePublicKey(blob)
	if err != nil || publicKey.Type() != ssh.KeyAlgoED25519 {
		return fmt.Errorf("public key blob is not Ed25519")
	}
	if string(ssh.MarshalAuthorizedKey(publicKey)) != publicText+"\n" {
		return fmt.Errorf("public key encoding is not canonical")
	}
	if err := validateFingerprint(fingerprint); err != nil {
		return err
	}
	if ssh.FingerprintSHA256(publicKey) != fingerprint {
		return fmt.Errorf("fingerprint does not match public key")
	}
	return nil
}

func validatePromotionKey(publicBase64, expectedSHA string) error {
	publicKey, err := decodeCanonicalBase64(publicBase64, ed25519.PublicKeySize)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("public key must be canonical base64 for %d bytes", ed25519.PublicKeySize)
	}
	if len(expectedSHA) != sha256.Size*2 || !isLowerHex(expectedSHA) {
		return fmt.Errorf("SHA-256 must be 64 lowercase hexadecimal characters")
	}
	digest := sha256.Sum256(publicKey)
	if hex.EncodeToString(digest[:]) != expectedSHA {
		return fmt.Errorf("SHA-256 does not match public key")
	}
	return nil
}
