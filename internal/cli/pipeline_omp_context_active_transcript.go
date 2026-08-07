package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

const (
	pipelineOMPActiveTranscriptMaxMessages = 4096
	pipelineOMPActiveTranscriptMaxBytes    = 64 << 20
	pipelineOMPActiveTranscriptPageSize    = 128
)

type pipelineOMPActiveMessagesPage struct {
	Messages      []json.RawMessage `json:"messages"`
	TotalMessages int               `json:"totalMessages"`
	NextCursor    *string           `json:"nextCursor"`
}

func (protocol *pipelineOMPRPCProtocol) validatePipelineOMPActiveTranscript(
	ctx context.Context,
	allowNewImages bool,
) (string, []string, error) {
	if protocol == nil || protocol.process == nil {
		return "", nil, errors.New("managed active OMP transcript authority is unavailable")
	}
	cursor, expectedTotal, count, bytesRead := "", -1, 0, 0
	all := make([]byte, 0, 4096)
	newImages := make([]string, 0)
	seenCursors := map[string]struct{}{"": {}}
	for {
		data, err := protocol.call(ctx, pipelineOMPRPCCommand{
			Type: "get_messages_page", Cursor: cursor, Limit: pipelineOMPActiveTranscriptPageSize,
		}, false)
		if err != nil {
			return "", nil, errors.New("managed active OMP transcript snapshot is unavailable")
		}
		var page pipelineOMPActiveMessagesPage
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&page) != nil || page.TotalMessages < 0 ||
			page.TotalMessages > pipelineOMPActiveTranscriptMaxMessages || len(page.Messages) > pipelineOMPActiveTranscriptPageSize {
			return "", nil, errors.New("managed active OMP transcript page is invalid")
		}
		if expectedTotal < 0 {
			expectedTotal = page.TotalMessages
		} else if page.TotalMessages != expectedTotal {
			return "", nil, errors.New("managed active OMP transcript snapshot changed")
		}
		for _, raw := range page.Messages {
			bytesRead += len(raw)
			if len(raw) == 0 || bytesRead > pipelineOMPActiveTranscriptMaxBytes || rejectDuplicatePipelineOMPJSON(raw) != nil {
				return "", nil, errors.New("managed active OMP transcript message is invalid")
			}
			var message any
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			if decoder.Decode(&message) != nil {
				return "", nil, errors.New("managed active OMP transcript message is invalid")
			}
			images, err := protocol.validatePipelineOMPActiveMessageValue(message, allowNewImages)
			if err != nil {
				return "", nil, err
			}
			newImages = append(newImages, images...)
			all = append(all, raw...)
			all = append(all, '\n')
			count++
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		if _, duplicate := seenCursors[*page.NextCursor]; duplicate {
			return "", nil, errors.New("managed active OMP transcript cursor repeated")
		}
		seenCursors[*page.NextCursor] = struct{}{}
		cursor = *page.NextCursor
	}
	if count != expectedTotal {
		return "", nil, errors.New("managed active OMP transcript page count is incomplete")
	}
	return pipelineOMPActiveHash(all), newImages, nil
}

func (protocol *pipelineOMPRPCProtocol) validatePipelineOMPActiveMessageValue(
	value any,
	allowNewImages bool,
) ([]string, error) {
	images := make([]string, 0)
	var walk func(any) error
	walk = func(current any) error {
		switch typed := current.(type) {
		case nil, bool, json.Number:
			return nil
		case string:
			return validatePipelineOMPActiveText(typed)
		case []any:
			for _, item := range typed {
				if err := walk(item); err != nil {
					return err
				}
			}
			return nil
		case map[string]any:
			if typeName, _ := typed["type"].(string); typeName == "image" {
				image, found := pipelineOMPActiveImageDigest(typed)
				if !found {
					return errors.New("managed active OMP transcript image is invalid")
				}
				if _, safe := protocol.safeCompactionImages[image]; !safe {
					if !allowNewImages {
						return errors.New("managed active OMP transcript image lacks compaction provenance")
					}
					images = append(images, image)
				}
				return nil
			}
			for key, item := range typed {
				if err := validatePipelineOMPActiveText(key); err != nil {
					return err
				}
				if strings.Contains(strings.ToLower(key), "image") {
					imageList, ok := item.([]any)
					role, _ := typed["role"].(string)
					if key != "images" || role != "compactionSummary" || !ok {
						return errors.New("managed active OMP transcript contains unsupported image content")
					}
					for _, imageValue := range imageList {
						image, imageOK := imageValue.(map[string]any)
						imageType, _ := image["type"].(string)
						if !imageOK || imageType != "image" {
							return errors.New("managed active OMP transcript contains unsupported image content")
						}
						if err := walk(image); err != nil {
							return err
						}
					}
					continue
				}
				if err := walk(item); err != nil {
					return err
				}
			}
			return nil
		default:
			return fmt.Errorf("managed active OMP transcript contains unsupported content %T", current)
		}
	}
	return images, walk(value)
}

func pipelineOMPActiveImageDigest(value map[string]any) (string, bool) {
	typeName, _ := value["type"].(string)
	if typeName != "image" {
		return "", false
	}
	data, dataOK := value["data"].(string)
	mime, mimeOK := value["mimeType"].(string)
	detailValue, hasDetail := value["detail"]
	detail, detailOK := detailValue.(string)
	if len(value) != 3+boolToPipelineOMPCount(hasDetail) || !dataOK || !mimeOK ||
		data == "" || mime != "image/png" || len(data) > pipelineOMPActiveTranscriptMaxBytes ||
		hasDetail && (!detailOK || !validPipelineOMPActiveImageDetail(detail)) {
		return "", false
	}
	for key := range value {
		if key != "type" && key != "data" && key != "mimeType" && key != "detail" {
			return "", false
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", false
	}
	return pipelineOMPActiveHash(encoded), true
}

func validPipelineOMPActiveImageDetail(detail string) bool {
	switch detail {
	case "auto", "low", "high", "original":
		return true
	default:
		return false
	}
}

func validatePipelineOMPActiveText(value string) error {
	sanitized := promptlayer.SanitizeContent(value, promptlayer.ContextOptions{
		MaxBytes: len(value)*2 + 1024, Required: true,
	})
	if sanitized.RedactionStatus != promptlayer.RedactionPassed || sanitized.Content != value {
		return errors.New("managed active OMP content failed sanitizer exact-pass")
	}
	return nil
}
