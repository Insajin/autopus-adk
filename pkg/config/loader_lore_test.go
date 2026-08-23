package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

// LoadLoreSection이 존재하는 이유가 이 테스트다. 커밋 메시지 검증은 lore 블록
// 두 값만 필요한데, 예전에는 전체 HarnessConfig.Validate를 통과해야 했다.
// 그래서 lore와 무관한 필드 하나가 커밋 훅 전체를 막았다 - platforms에 omp가
// 추가되자 그것을 모르는 구버전 바이너리에서 커밋이 불가능해졌다.
func TestLoadLoreSection_IgnoresUnrelatedInvariants(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// platforms가 유효하지 않아도, mode가 비어도 lore 블록은 읽혀야 한다.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(
		"project_name: probe\n"+
			"platforms:\n  - definitely-not-a-platform\n"+
			"lore:\n  required_trailers: [Constraint, Confidence]\n  stale_threshold_days: 7\n",
	), 0o600))

	loaded, err := config.LoadLoreSection(dir)
	require.NoError(t, err, "lore 검증은 무관한 설정 불변식에 의존해서는 안 된다")
	assert.Equal(t, []string{"Constraint", "Confidence"}, loaded.RequiredTrailers)
	assert.Equal(t, 7, loaded.StaleThresholdDays)

	// 대조: 같은 설정으로 전체 로드는 여전히 실패해야 한다. 설정 유효성은
	// 사라진 것이 아니라 doctor가 소유한다.
	_, fullErr := config.LoadPreview(dir)
	require.Error(t, fullErr, "전체 설정 검증이 느슨해지면 안 된다")
}

// 파일이 없거나 lore 블록이 생략되면 기본값을 준다. 생략된 키를 0 값으로 두면
// 트레일러 요구가 조용히 사라진다.
func TestLoadLoreSection_FillsOmittedKeysFromDefaults(t *testing.T) {
	t.Parallel()

	defaults := config.DefaultFullConfig("probe").Lore

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		loaded, err := config.LoadLoreSection(t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, defaults.RequiredTrailers, loaded.RequiredTrailers)
		assert.Equal(t, defaults.StaleThresholdDays, loaded.StaleThresholdDays)
	})

	t.Run("block omitted", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "autopus.yaml"),
			[]byte("mode: full\nproject_name: probe\nplatforms:\n  - claude-code\n"), 0o600))

		loaded, err := config.LoadLoreSection(dir)
		require.NoError(t, err)
		assert.Equal(t, defaults.RequiredTrailers, loaded.RequiredTrailers)
		assert.Equal(t, defaults.StaleThresholdDays, loaded.StaleThresholdDays)
	})

	t.Run("keys partially omitted", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "autopus.yaml"),
			[]byte("mode: full\nproject_name: probe\nplatforms:\n  - claude-code\nlore:\n  enabled: true\n"), 0o600))

		loaded, err := config.LoadLoreSection(dir)
		require.NoError(t, err)
		assert.Equal(t, defaults.RequiredTrailers, loaded.RequiredTrailers,
			"생략된 required_trailers는 기본값으로 되돌아가야 한다")
		assert.Equal(t, defaults.StaleThresholdDays, loaded.StaleThresholdDays)
	})
}

// 깨진 YAML은 실패해야 한다. 그때 트레일러 요구를 조용히 낮추는 것보다 멈추는
// 편이 옳다.
func TestLoadLoreSection_MalformedYAMLFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "autopus.yaml"),
		[]byte("lore: [this is not a mapping\n"), 0o600))

	_, err := config.LoadLoreSection(dir)
	require.Error(t, err)
}

// check 경로는 검사 대상을 수정하지 않는다. Load는 플랫폼 이름 정규화 결과를
// autopus.yaml에 기록하므로, 훅에서 도는 경로는 LoadPreview를 써야 한다.
func TestLoadLoreSection_DoesNotRewriteConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "autopus.yaml")
	// gemini-cli는 antigravity-cli로 정규화되는 이름이다.
	original := "mode: full\nproject_name: probe\nplatforms:\n  - gemini-cli\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o600))

	_, err := config.LoadLoreSection(dir)
	require.NoError(t, err)

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(after), "lore 로드가 설정 파일을 다시 쓰면 안 된다")
}
