package adkchannel

import "time"

const (
	rotationSchema                       = "adk-key-rotation.v1"
	rotationBridgeTag                    = "v0.50.109"
	rotationReleaseMode                  = "canonical-full-bridge"
	rotationSignatureDomain              = "autopus.adk-channel.key-rotation.v1\x00"
	rotationPreviousTagFingerprint       = "SHA256:bhW+YA+FZ6G4d9Z8BM/eBss6l0I/fcVmV7k986GupK0"
	rotationNextTagPublicKey             = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPKdXtl0E+TcLmC94idkTgtM5XUA5UqP9An0vNFp0FlY"
	rotationNextTagFingerprint           = "SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ"
	rotationNextPromotionKeyID           = "omp-context-promotion-2026-q3-k3"
	rotationNextPromotionPublicKey       = "YkTuNcfWGTLgTglPmZq/Dj4OXwcoUwnkM2ExIGIz+jM="
	rotationNextPromotionPublicKeySHA256 = "2a9b41dec1330f65937d9b25b20967cb29fd9209c722ce5fe1a9afd6ca45b937"
	rotationMaxValidity                  = 24 * time.Hour
)

type RotationOptions struct {
	PublicKeyBase64      string
	ExpectedKeyID        string
	ExpectedSourceCommit string
	ExpectedSourceTree   string
	Now                  func() time.Time
}

type RotationPins struct {
	NextTagPublicKey       string
	NextTagFingerprint     string
	NextPromotionPublicKey string
}

type RotationPinFiles struct {
	NextTagPublicKeyPath       string
	NextTagFingerprintPath     string
	NextPromotionPublicKeyPath string
}

type RotationDocument struct {
	SchemaVersion                string `json:"schema_version"`
	Channel                      string `json:"channel"`
	Repository                   string `json:"repository"`
	BridgeTag                    string `json:"bridge_tag"`
	ReleaseMode                  string `json:"release_mode"`
	SourceCommit                 string `json:"source_commit"`
	SourceTree                   string `json:"source_tree"`
	IssuedAt                     string `json:"issued_at"`
	ExpiresAt                    string `json:"expires_at"`
	ChannelKeyID                 string `json:"channel_key_id"`
	PreviousTagFingerprint       string `json:"previous_tag_fingerprint"`
	NextTagPublicKey             string `json:"next_tag_public_key"`
	NextTagFingerprint           string `json:"next_tag_fingerprint"`
	NextPromotionKeyID           string `json:"next_promotion_key_id"`
	NextPromotionPublicKey       string `json:"next_promotion_public_key"`
	NextPromotionPublicKeySHA256 string `json:"next_promotion_public_key_sha256"`
}

type RotationVerified struct {
	Document          RotationDocument
	CanonicalDocument []byte
}
