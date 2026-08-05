package omp

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func encodeOMPConfigRollbackArtifact(before, after []byte) ([]byte, error) {
	beforeLines, beforeOffsets := splitOMPConfigRollbackLines(before)
	afterLines, afterOffsets := splitOMPConfigRollbackLines(after)
	var matches []ompConfigRollbackMatch
	if len(beforeLines) != 0 && len(afterLines) != 0 {
		var err error
		matches, err = ompConfigRollbackMatchingBlocks(beforeLines, afterLines)
		if err != nil {
			return nil, err
		}
	}
	artifact := ompConfigRollbackArtifact{
		Schema:         ompConfigRollbackSchema,
		BeforeChecksum: adapter.Checksum(string(before)),
		AfterChecksum:  adapter.Checksum(string(after)),
		Hunks:          make([]ompConfigRollbackHunk, 0),
	}
	beforeCursor, afterCursor := 0, 0
	for _, match := range append(matches, ompConfigRollbackMatch{
		before: len(beforeLines), after: len(afterLines),
	}) {
		beforeBlock := before[beforeOffsets[beforeCursor]:beforeOffsets[match.before]]
		afterBlock := after[afterOffsets[afterCursor]:afterOffsets[match.after]]
		hunks, err := diffOMPConfigRollbackBlock(
			beforeBlock, afterBlock, afterOffsets[afterCursor],
		)
		if err != nil {
			return nil, err
		}
		artifact.Hunks = append(artifact.Hunks, hunks...)
		beforeCursor = match.before + match.length
		afterCursor = match.after + match.length
	}
	data, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	if len(data)+1 > maxOMPConfigRollbackArtifact {
		return nil, fmt.Errorf("rollback artifact exceeds %d bytes", maxOMPConfigRollbackArtifact)
	}
	return append(data, '\n'), nil
}

func splitOMPConfigRollbackLines(data []byte) ([]string, []int) {
	lines := make([]string, 0, bytes.Count(data, []byte{'\n'})+1)
	offsets := []int{0}
	start := 0
	for index, value := range data {
		if value != '\n' {
			continue
		}
		lines = append(lines, string(data[start:index+1]))
		offsets = append(offsets, index+1)
		start = index + 1
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
		offsets = append(offsets, len(data))
	}
	return lines, offsets
}

func diffOMPConfigRollbackBlock(
	before []byte,
	after []byte,
	afterOffset int,
) ([]ompConfigRollbackHunk, error) {
	prefix := 0
	for prefix < len(before) && prefix < len(after) && before[prefix] == after[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(before)-prefix && suffix < len(after)-prefix &&
		before[len(before)-1-suffix] == after[len(after)-1-suffix] {
		suffix++
	}
	before = before[prefix : len(before)-suffix]
	after = after[prefix : len(after)-suffix]
	afterOffset += prefix
	if len(before) == 0 && len(after) == 0 {
		return nil, nil
	}
	if len(before) == 0 || len(after) == 0 {
		return []ompConfigRollbackHunk{{
			Offset: afterOffset, AfterLength: len(after),
			Before: append([]byte(nil), before...),
		}}, nil
	}
	if len(before)+len(after) > maxOMPConfigRollbackDiffBlock {
		return nil, fmt.Errorf("rollback diff block exceeds %d bytes", maxOMPConfigRollbackDiffBlock)
	}
	beforeBytes := ompConfigRollbackByteSequence(before)
	afterBytes := ompConfigRollbackByteSequence(after)
	matches, err := ompConfigRollbackMatchingBlocks(beforeBytes, afterBytes)
	if err != nil {
		return nil, err
	}
	hunks := make([]ompConfigRollbackHunk, 0)
	beforeCursor, afterCursor := 0, 0
	for _, match := range append(matches, ompConfigRollbackMatch{
		before: len(before), after: len(after),
	}) {
		if beforeCursor != match.before || afterCursor != match.after {
			hunks = append(hunks, ompConfigRollbackHunk{
				Offset:      afterOffset + afterCursor,
				AfterLength: match.after - afterCursor,
				Before:      append([]byte(nil), before[beforeCursor:match.before]...),
			})
		}
		beforeCursor = match.before + match.length
		afterCursor = match.after + match.length
	}
	return hunks, nil
}

type ompConfigRollbackMatch struct {
	before int
	after  int
	length int
}

func ompConfigRollbackMatchingBlocks(
	before []string,
	after []string,
) ([]ompConfigRollbackMatch, error) {
	maxDistance := len(before) + len(after)
	if maxDistance > maxOMPConfigRollbackEditDistance {
		maxDistance = maxOMPConfigRollbackEditDistance
	}
	frontier := map[int]int{1: 0}
	trace := make([]map[int]int, 0, maxDistance+1)
	finalDistance := -1
	for distance := 0; distance <= maxDistance; distance++ {
		next := make(map[int]int, distance+1)
		for diagonal := -distance; diagonal <= distance; diagonal += 2 {
			var x int
			if diagonal == -distance ||
				diagonal != distance && frontier[diagonal-1] < frontier[diagonal+1] {
				x = frontier[diagonal+1]
			} else {
				x = frontier[diagonal-1] + 1
			}
			y := x - diagonal
			for x < len(before) && y < len(after) && before[x] == after[y] {
				x++
				y++
			}
			next[diagonal] = x
			if x == len(before) && y == len(after) {
				finalDistance = distance
				break
			}
		}
		trace = append(trace, next)
		frontier = next
		if finalDistance >= 0 {
			break
		}
	}
	if finalDistance < 0 {
		return nil, fmt.Errorf(
			"rollback edit distance exceeds %d", maxOMPConfigRollbackEditDistance,
		)
	}

	x, y := len(before), len(after)
	reversed := make([]ompConfigRollbackMatch, 0)
	for distance := finalDistance; distance > 0; distance-- {
		previous := trace[distance-1]
		diagonal := x - y
		var previousDiagonal int
		if diagonal == -distance ||
			diagonal != distance && previous[diagonal-1] < previous[diagonal+1] {
			previousDiagonal = diagonal + 1
		} else {
			previousDiagonal = diagonal - 1
		}
		previousX := previous[previousDiagonal]
		previousY := previousX - previousDiagonal
		for x > previousX && y > previousY {
			x--
			y--
			reversed = append(reversed, ompConfigRollbackMatch{
				before: x, after: y, length: 1,
			})
		}
		x, y = previousX, previousY
	}
	for x > 0 && y > 0 {
		x--
		y--
		if before[x] != after[y] {
			return nil, fmt.Errorf("rollback diff backtrack invalid")
		}
		reversed = append(reversed, ompConfigRollbackMatch{
			before: x, after: y, length: 1,
		})
	}
	matches := make([]ompConfigRollbackMatch, 0, len(reversed))
	for index := len(reversed) - 1; index >= 0; index-- {
		match := reversed[index]
		if len(matches) != 0 {
			last := &matches[len(matches)-1]
			if last.before+last.length == match.before &&
				last.after+last.length == match.after {
				last.length++
				continue
			}
		}
		matches = append(matches, match)
	}
	return matches, nil
}

var ompConfigRollbackByteValues = func() [256]string {
	var values [256]string
	for index := range values {
		values[index] = string([]byte{byte(index)})
	}
	return values
}()

func ompConfigRollbackByteSequence(data []byte) []string {
	values := make([]string, len(data))
	for index, value := range data {
		values[index] = ompConfigRollbackByteValues[value]
	}
	return values
}

func decodeOMPConfigRollbackArtifact(data []byte) (ompConfigRollbackArtifact, error) {
	if len(data) > maxOMPConfigRollbackArtifact {
		return ompConfigRollbackArtifact{}, fmt.Errorf("rollback artifact exceeds size limit")
	}
	var artifact ompConfigRollbackArtifact
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&artifact); err != nil || requireOMPModelDoctorJSONEOF(decoder) != nil {
		return ompConfigRollbackArtifact{}, fmt.Errorf("rollback artifact invalid")
	}
	if artifact.Schema != ompConfigRollbackSchema ||
		artifact.BeforeChecksum == "" || artifact.AfterChecksum == "" {
		return ompConfigRollbackArtifact{}, fmt.Errorf("rollback artifact invalid")
	}
	cursor := 0
	for _, hunk := range artifact.Hunks {
		if hunk.Offset < cursor || hunk.AfterLength < 0 {
			return ompConfigRollbackArtifact{}, fmt.Errorf("rollback artifact invalid")
		}
		cursor = hunk.Offset + hunk.AfterLength
	}
	return artifact, nil
}

func applyOMPConfigRollbackArtifact(
	artifact ompConfigRollbackArtifact,
	current []byte,
) ([]byte, error) {
	if adapter.Checksum(string(current)) != artifact.AfterChecksum {
		return nil, fmt.Errorf("rollback artifact after checksum mismatch")
	}
	result := make([]byte, 0, len(current))
	cursor := 0
	for _, hunk := range artifact.Hunks {
		if hunk.Offset < cursor || hunk.Offset > len(current) ||
			hunk.AfterLength > len(current)-hunk.Offset {
			return nil, fmt.Errorf("rollback artifact range invalid")
		}
		result = append(result, current[cursor:hunk.Offset]...)
		result = append(result, hunk.Before...)
		cursor = hunk.Offset + hunk.AfterLength
	}
	result = append(result, current[cursor:]...)
	if adapter.Checksum(string(result)) != artifact.BeforeChecksum {
		return nil, fmt.Errorf("rollback artifact before checksum mismatch")
	}
	return result, nil
}
