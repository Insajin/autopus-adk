package companionmanifest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
)

const publicKeyReceiptRecordDomain = "autopus.public-key-receipt.a0-record.v1\x00"

const (
	configuredA0ReceiptSHA256   = "sha256:4a588fa4991c515e9520861af5567fd2fe4c19e2c23adb8963bd37ebc46a5bbc"
	configuredA0SignatureSHA256 = "sha256:7f248929d807b689acab575888b0a7600bd2ea17cce1e5fcc11f72af9c510173"
	configuredA0PublicKeySHA256 = "sha256:c387da9e9c43dbaa2605207a00635c84937ff397a8b6ed73414d2e66b89941a4"
	configuredA0RecordSHA256    = "sha256:84ee9403223aabd1f60e5e55e79a5c7d6b2c764bc594435cbf7c4e997e2ce475"
)

// ErrPublicKeyReceiptA0Unprovisioned means immutable release pins are absent.
var ErrPublicKeyReceiptA0Unprovisioned = errors.New(
	"public key receipt A0 anchor is unprovisioned",
)

type publicKeyReceiptA0Anchor struct {
	receiptDigest   [sha256.Size]byte
	signatureDigest [sha256.Size]byte
	publicKeyDigest [sha256.Size]byte
	recordDigest    [sha256.Size]byte
}

type publicKeyReceiptBundleRecord struct {
	receiptBytes []byte
	signature    []byte
}

// TrustedPublicKeyReceipt is an opaque release-trust capability.
// Its zero value is invalid and callers cannot construct a valid value.
type TrustedPublicKeyReceipt struct {
	receipt          PublicKeyReceipt
	publicKey        [ed25519.PublicKeySize]byte
	receiptDigest    [sha256.Size]byte
	signatureDigest  [sha256.Size]byte
	publicKeyDigest  [sha256.Size]byte
	recordDigest     [sha256.Size]byte
	capabilityDigest [sha256.Size]byte
}

func newPublicKeyReceiptA0Anchor(
	receiptDigest,
	signatureDigest,
	publicKeyDigest,
	recordDigest [sha256.Size]byte,
) (publicKeyReceiptA0Anchor, error) {
	anchor := publicKeyReceiptA0Anchor{
		receiptDigest: receiptDigest, signatureDigest: signatureDigest,
		publicKeyDigest: publicKeyDigest, recordDigest: recordDigest,
	}
	if !anchor.valid() {
		return publicKeyReceiptA0Anchor{}, errors.New("empty public key receipt A0 anchor")
	}
	return anchor, nil
}

func configuredPublicKeyReceiptA0Anchor() (publicKeyReceiptA0Anchor, error) {
	return publicKeyReceiptA0AnchorFromPins([...]string{
		configuredA0ReceiptSHA256,
		configuredA0SignatureSHA256,
		configuredA0PublicKeySHA256,
		configuredA0RecordSHA256,
	})
}

func publicKeyReceiptA0AnchorFromPins(
	pins [4]string,
) (publicKeyReceiptA0Anchor, error) {
	for _, pin := range pins {
		if pin == "" {
			return publicKeyReceiptA0Anchor{}, ErrPublicKeyReceiptA0Unprovisioned
		}
	}
	receiptDigest, err := decodePublicKeyReceiptA0Pin(pins[0])
	if err != nil {
		return publicKeyReceiptA0Anchor{}, err
	}
	signatureDigest, err := decodePublicKeyReceiptA0Pin(pins[1])
	if err != nil {
		return publicKeyReceiptA0Anchor{}, err
	}
	publicKeyDigest, err := decodePublicKeyReceiptA0Pin(pins[2])
	if err != nil {
		return publicKeyReceiptA0Anchor{}, err
	}
	recordDigest, err := decodePublicKeyReceiptA0Pin(pins[3])
	if err != nil {
		return publicKeyReceiptA0Anchor{}, err
	}
	return newPublicKeyReceiptA0Anchor(
		receiptDigest,
		signatureDigest,
		publicKeyDigest,
		recordDigest,
	)
}

func decodePublicKeyReceiptA0Pin(value string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+sha256.Size*2 {
		return digest, errors.New("invalid configured public key receipt A0 pin")
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	if err != nil || len(decoded) != sha256.Size {
		return digest, errors.New("invalid configured public key receipt A0 pin")
	}
	copy(digest[:], decoded)
	return digest, nil
}

func (anchor publicKeyReceiptA0Anchor) valid() bool {
	return !zeroDigest(anchor.receiptDigest) &&
		!zeroDigest(anchor.signatureDigest) &&
		!zeroDigest(anchor.publicKeyDigest) &&
		!zeroDigest(anchor.recordDigest)
}

func publicKeyReceiptRecordDigest(receiptBytes, signature []byte) [sha256.Size]byte {
	receiptDigest := sha256.Sum256(receiptBytes)
	signatureDigest := sha256.Sum256(signature)
	hash := sha256.New()
	_, _ = hash.Write([]byte(publicKeyReceiptRecordDomain))
	_, _ = hash.Write(receiptDigest[:])
	_, _ = hash.Write(signatureDigest[:])
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func verifyPublicKeyReceiptA0Record(
	record publicKeyReceiptBundleRecord,
	policy PublicKeyReceiptPolicy,
	anchor publicKeyReceiptA0Anchor,
) (TrustedPublicKeyReceipt, error) {
	if !anchor.valid() {
		return TrustedPublicKeyReceipt{}, errors.New("missing public key receipt A0 anchor")
	}
	receipt, publicKey, err := checkPublicKeyReceiptSelfConsistency(
		record.receiptBytes,
		record.signature,
		policy,
	)
	if err != nil {
		return TrustedPublicKeyReceipt{}, err
	}
	receiptDigest := sha256.Sum256(record.receiptBytes)
	signatureDigest := sha256.Sum256(record.signature)
	publicKeyDigest := sha256.Sum256(publicKey)
	recordDigest := publicKeyReceiptRecordDigest(record.receiptBytes, record.signature)
	if !sameDigest(receiptDigest, anchor.receiptDigest) ||
		!sameDigest(signatureDigest, anchor.signatureDigest) ||
		!sameDigest(publicKeyDigest, anchor.publicKeyDigest) ||
		!sameDigest(recordDigest, anchor.recordDigest) {
		return TrustedPublicKeyReceipt{}, errors.New("public key receipt A0 anchor mismatch")
	}
	trusted := TrustedPublicKeyReceipt{
		receipt: receipt, receiptDigest: receiptDigest,
		signatureDigest: signatureDigest, publicKeyDigest: publicKeyDigest,
		recordDigest: recordDigest,
	}
	copy(trusted.publicKey[:], publicKey)
	trusted.capabilityDigest = trustedPublicKeyReceiptCapabilityDigest(trusted)
	return trusted, nil
}

// Receipt returns immutable public claims without exposing signing key bytes.
func (trusted TrustedPublicKeyReceipt) Receipt() (PublicKeyReceipt, bool) {
	if !trusted.valid() {
		return PublicKeyReceipt{}, false
	}
	return trusted.receipt, true
}

func (trusted TrustedPublicKeyReceipt) valid() bool {
	if zeroDigest(trusted.capabilityDigest) || zeroDigest(trusted.publicKeyDigest) ||
		zeroDigest(trusted.recordDigest) {
		return false
	}
	want := trustedPublicKeyReceiptCapabilityDigest(trusted)
	return sameDigest(want, trusted.capabilityDigest)
}

func trustedPublicKeyReceiptCapabilityDigest(
	trusted TrustedPublicKeyReceipt,
) [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("autopus.public-key-receipt.trusted-capability.v1\x00"))
	_, _ = hash.Write(trusted.receiptDigest[:])
	_, _ = hash.Write(trusted.signatureDigest[:])
	_, _ = hash.Write(trusted.publicKeyDigest[:])
	_, _ = hash.Write(trusted.recordDigest[:])
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func zeroDigest(value [sha256.Size]byte) bool {
	var zero [sha256.Size]byte
	return subtle.ConstantTimeCompare(value[:], zero[:]) == 1
}

func sameDigest(left, right [sha256.Size]byte) bool {
	return subtle.ConstantTimeCompare(left[:], right[:]) == 1
}
