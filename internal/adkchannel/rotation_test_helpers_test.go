package adkchannel

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

type rotationFixture struct {
	now      time.Time
	private  ed25519.PrivateKey
	options  RotationOptions
	pins     RotationPins
	document RotationDocument
}

func newRotationFixture(t *testing.T) rotationFixture {
	t.Helper()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x51}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	pins := RotationPins{
		NextTagPublicKey:       rotationNextTagPublicKey,
		NextTagFingerprint:     rotationNextTagFingerprint,
		NextPromotionPublicKey: rotationNextPromotionPublicKey,
	}
	document := RotationDocument{
		SchemaVersion:                rotationSchema,
		Channel:                      channelName,
		Repository:                   channelRepository,
		BridgeTag:                    rotationBridgeTag,
		ReleaseMode:                  rotationReleaseMode,
		SourceCommit:                 "c2a50831c925a4b08d55991039ed3b1113392888",
		SourceTree:                   "cad3d28d8375363c9e4b536c98c39b8d51aaf999",
		IssuedAt:                     now.Add(-time.Hour).Format(time.RFC3339),
		ExpiresAt:                    now.Add(time.Hour).Format(time.RFC3339),
		ChannelKeyID:                 testKeyID,
		PreviousTagFingerprint:       rotationPreviousTagFingerprint,
		NextTagPublicKey:             pins.NextTagPublicKey,
		NextTagFingerprint:           pins.NextTagFingerprint,
		NextPromotionKeyID:           rotationNextPromotionKeyID,
		NextPromotionPublicKey:       pins.NextPromotionPublicKey,
		NextPromotionPublicKeySHA256: rotationNextPromotionPublicKeySHA256,
	}
	return rotationFixture{
		now:     now,
		private: private,
		options: RotationOptions{
			PublicKeyBase64:      base64.StdEncoding.EncodeToString(public),
			ExpectedKeyID:        testKeyID,
			ExpectedSourceCommit: document.SourceCommit,
			ExpectedSourceTree:   document.SourceTree,
			Now:                  func() time.Time { return now },
		},
		pins:     pins,
		document: document,
	}
}

func signRotationDocument(t *testing.T, private ed25519.PrivateKey, document RotationDocument) SignedBytes {
	t.Helper()
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	message := append(append([]byte(nil), rotationSignatureDomain...), encoded...)
	return SignedBytes{Document: encoded, Signature: ed25519.Sign(private, message)}
}
