package config

import (
	"errors"
	"fmt"
	"strings"
)

const (
	MaxCodexModelCatalogBytes      = 4 << 20
	MaxCodexCatalogModels          = 256
	MaxCodexCatalogSlugBytes       = 256
	MaxCodexCatalogReasoningLevels = 16
	MaxCodexCatalogEffortBytes     = 32
)

// ValidateCodexModelCatalogPayload bounds catalog data before runtime profile resolution.
func ValidateCodexModelCatalogPayload(data []byte) error {
	_, err := ParseCodexModelCatalog(data)
	return err
}

func validateCodexModelCatalog(catalog CodexModelCatalog) error {
	if len(catalog.Models) == 0 {
		return errors.New("codex model catalog has no models")
	}
	if len(catalog.Models) > MaxCodexCatalogModels {
		return fmt.Errorf("codex model catalog has %d models; limit is %d", len(catalog.Models), MaxCodexCatalogModels)
	}
	for modelIndex, model := range catalog.Models {
		if strings.TrimSpace(model.Slug) == "" {
			return fmt.Errorf("codex model catalog model %d has no slug", modelIndex)
		}
		if len(model.Slug) > MaxCodexCatalogSlugBytes {
			return fmt.Errorf("codex model catalog model %d slug exceeds %d bytes", modelIndex, MaxCodexCatalogSlugBytes)
		}
		if len(model.DefaultReasoningLevel) > MaxCodexCatalogEffortBytes {
			return fmt.Errorf("codex model catalog model %d default effort exceeds %d bytes", modelIndex, MaxCodexCatalogEffortBytes)
		}
		if len(model.SupportedReasoningLevels) > MaxCodexCatalogReasoningLevels {
			return fmt.Errorf(
				"codex model catalog model %d has %d reasoning levels; limit is %d",
				modelIndex,
				len(model.SupportedReasoningLevels),
				MaxCodexCatalogReasoningLevels,
			)
		}
		for effortIndex, level := range model.SupportedReasoningLevels {
			if len(level.Effort) > MaxCodexCatalogEffortBytes {
				return fmt.Errorf(
					"codex model catalog model %d effort %d exceeds %d bytes",
					modelIndex,
					effortIndex,
					MaxCodexCatalogEffortBytes,
				)
			}
		}
	}
	return nil
}
