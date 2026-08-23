package detect

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

// 이 테스트가 존재하는 이유는 회귀 하나다. cliVersionTimeout이 리터럴 5초였을
// 때, 공유 배수 유예를 250ms에서 2초로 올리자 프로브의 실제 응답 여유가 4.75초
// 에서 3초로 줄었다. 아무 테스트도 깨지지 않았다 - 여유가 남아 있었으므로.
// 그러다 부하가 걸린 레인에서 프로브가 상한을 꽉 채우고 버전이 "unknown"으로
// 저하되면서, 상한과 무관해 보이는 감지 테스트가 뒤집혔다.
//
// 그래서 두 경계의 관계를 고정한다. 응답 예산은 유예에 종속되지 않아야 한다.
func TestVersionCeilingLeavesTheResponseBudgetIntactAboveTheDrainGrace(t *testing.T) {
	assert.Greater(t, cliVersionTimeout, processprobe.DefaultWaitDelay,
		"version-probe ceiling must exceed processprobe.DefaultWaitDelay; "+
			"otherwise the post-exit drain consumes the whole ceiling")

	// 유예를 뺀 나머지가 실제 응답 예산이다. 이 값이 유예 변경에 흔들리면
	// 상수가 다시 리터럴로 돌아간 것이다.
	responseBudget := cliVersionTimeout - processprobe.DefaultWaitDelay
	assert.Equal(t, 5*time.Second, responseBudget,
		"a CLI must keep a fixed 5s to answer --version regardless of the drain grace")
}
