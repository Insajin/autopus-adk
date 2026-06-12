package spec_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestParseEARS_Ubiquitous(t *testing.T) {
	t.Parallel()

	text := "시스템은 SHALL 모든 요청을 처리합니다."
	reqs, err := spec.ParseEARS(text)
	require.NoError(t, err)

	require.Len(t, reqs, 1)
	assert.Equal(t, spec.EARSUbiquitous, reqs[0].Type)
	assert.NotEmpty(t, reqs[0].Description)
}

func TestParseEARS_EventDriven(t *testing.T) {
	t.Parallel()

	text := "WHEN 사용자가 로그인하면 THEN 시스템은 세션을 생성합니다."
	reqs, err := spec.ParseEARS(text)
	require.NoError(t, err)

	require.Len(t, reqs, 1)
	assert.Equal(t, spec.EARSEventDriven, reqs[0].Type)
}

func TestParseEARS_StateDriven(t *testing.T) {
	t.Parallel()

	text := "WHERE 시스템이 유지보수 모드인 경우 THEN 새 요청을 거부합니다."
	reqs, err := spec.ParseEARS(text)
	require.NoError(t, err)

	require.Len(t, reqs, 1)
	assert.Equal(t, spec.EARSStateDriven, reqs[0].Type)
}

func TestParseEARS_Unwanted(t *testing.T) {
	t.Parallel()

	text := "IF 네트워크 연결이 실패하면 THEN 시스템은 재시도합니다."
	reqs, err := spec.ParseEARS(text)
	require.NoError(t, err)

	require.Len(t, reqs, 1)
	assert.Equal(t, spec.EARSUnwanted, reqs[0].Type)
}

func TestParseEARS_Optional(t *testing.T) {
	t.Parallel()

	text := "WHEN 사용자가 요청하고 IF 권한이 있으면 THEN 시스템은 데이터를 반환합니다."
	reqs, err := spec.ParseEARS(text)
	require.NoError(t, err)

	require.Len(t, reqs, 1)
	assert.Equal(t, spec.EARSOptional, reqs[0].Type)
}

func TestParseEARS_MultipleRequirements(t *testing.T) {
	t.Parallel()

	text := `## Requirements

시스템은 SHALL 데이터를 저장합니다.

WHEN 저장이 완료되면 THEN 알림을 보냅니다.

IF 오류가 발생하면 THEN 롤백합니다.
`
	reqs, err := spec.ParseEARS(text)
	require.NoError(t, err)

	assert.Len(t, reqs, 3)

	types := make(map[spec.EARSType]bool)
	for _, r := range reqs {
		types[r.Type] = true
	}
	assert.True(t, types[spec.EARSUbiquitous])
	assert.True(t, types[spec.EARSEventDriven])
	assert.True(t, types[spec.EARSUnwanted])
}

func TestParseEARS_IDAssignment(t *testing.T) {
	t.Parallel()

	text := `시스템은 SHALL 첫 번째 기능을 제공합니다.
시스템은 SHALL 두 번째 기능을 제공합니다.
`
	reqs, err := spec.ParseEARS(text)
	require.NoError(t, err)

	require.Len(t, reqs, 2)
	assert.Equal(t, "REQ-001", reqs[0].ID)
	assert.Equal(t, "REQ-002", reqs[1].ID)
}

func TestParseEARS_NoRequirements(t *testing.T) {
	t.Parallel()

	text := "이것은 일반 텍스트입니다. 요구사항 패턴이 없습니다."
	reqs, err := spec.ParseEARS(text)
	require.NoError(t, err)
	assert.Empty(t, reqs)
}

func TestParseEARS_EnglishPatterns(t *testing.T) {
	t.Parallel()

	text := "The system SHALL process all requests."
	reqs, err := spec.ParseEARS(text)
	require.NoError(t, err)
	require.Len(t, reqs, 1)
	assert.Equal(t, spec.EARSUbiquitous, reqs[0].Type)
}

// S7 (SPEC-SPECREV-002 REQ-004): a requirement-looking SHALL line that no EARS
// type recognizes yields no requirement but a single warning naming the line.
func TestParseEARSWithWarnings_UnrecognizedShallLineWarns(t *testing.T) {
	t.Parallel()

	text := "The button SHALL respond to the click"
	reqs, warnings, err := spec.ParseEARSWithWarnings(text)
	require.NoError(t, err)
	require.Len(t, reqs, 0)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "The button SHALL respond to the click")
}

// S8 (SPEC-SPECREV-002 REQ-004): a recognized EARS line produces a requirement
// and no warning.
func TestParseEARSWithWarnings_RecognizedLineNoWarning(t *testing.T) {
	t.Parallel()

	text := "WHEN the user clicks THEN the system SHALL respond"
	reqs, warnings, err := spec.ParseEARSWithWarnings(text)
	require.NoError(t, err)
	require.Len(t, reqs, 1)
	assert.Equal(t, spec.EARSEventDriven, reqs[0].Type)
	require.Len(t, warnings, 0)
}
