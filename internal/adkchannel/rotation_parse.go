package adkchannel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const (
	rotationFieldCount = 16
	rotationAllFields  = uint16(0xffff)
)

func parseRotationDocument(encoded []byte) (RotationDocument, error) {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return RotationDocument{}, fmt.Errorf("document must be a JSON object")
	}

	var document RotationDocument
	var seen uint16
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return RotationDocument{}, fmt.Errorf("document field is invalid: %w", err)
		}
		name, ok := token.(string)
		if !ok {
			return RotationDocument{}, fmt.Errorf("document field name is invalid")
		}

		var value string
		if err := decoder.Decode(&value); err != nil {
			return RotationDocument{}, fmt.Errorf("field %q must be a string", name)
		}
		field, err := assignRotationField(&document, name, value)
		if err != nil {
			return RotationDocument{}, err
		}
		if seen&field != 0 {
			return RotationDocument{}, fmt.Errorf("duplicate field %q", name)
		}
		seen |= field
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return RotationDocument{}, fmt.Errorf("document object is not closed")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return RotationDocument{}, fmt.Errorf("document has trailing JSON")
	}
	if seen != rotationAllFields {
		return RotationDocument{}, fmt.Errorf("document must contain exactly %d fields", rotationFieldCount)
	}
	canonical, err := json.Marshal(document)
	if err != nil {
		return RotationDocument{}, fmt.Errorf("encode canonical document: %w", err)
	}
	if !bytes.Equal(canonical, encoded) {
		return RotationDocument{}, fmt.Errorf("document JSON is not canonical")
	}
	return document, nil
}

func assignRotationField(document *RotationDocument, name, value string) (uint16, error) {
	switch name {
	case "schema_version":
		document.SchemaVersion = value
		return 1 << 0, nil
	case "channel":
		document.Channel = value
		return 1 << 1, nil
	case "repository":
		document.Repository = value
		return 1 << 2, nil
	case "bridge_tag":
		document.BridgeTag = value
		return 1 << 3, nil
	case "release_mode":
		document.ReleaseMode = value
		return 1 << 4, nil
	case "source_commit":
		document.SourceCommit = value
		return 1 << 5, nil
	case "source_tree":
		document.SourceTree = value
		return 1 << 6, nil
	case "issued_at":
		document.IssuedAt = value
		return 1 << 7, nil
	case "expires_at":
		document.ExpiresAt = value
		return 1 << 8, nil
	case "channel_key_id":
		document.ChannelKeyID = value
		return 1 << 9, nil
	case "previous_tag_fingerprint":
		document.PreviousTagFingerprint = value
		return 1 << 10, nil
	case "next_tag_public_key":
		document.NextTagPublicKey = value
		return 1 << 11, nil
	case "next_tag_fingerprint":
		document.NextTagFingerprint = value
		return 1 << 12, nil
	case "next_promotion_key_id":
		document.NextPromotionKeyID = value
		return 1 << 13, nil
	case "next_promotion_public_key":
		document.NextPromotionPublicKey = value
		return 1 << 14, nil
	case "next_promotion_public_key_sha256":
		document.NextPromotionPublicKeySHA256 = value
		return 1 << 15, nil
	default:
		return 0, fmt.Errorf("unknown field %q", name)
	}
}
