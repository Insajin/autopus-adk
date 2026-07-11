# SPEC-CODEXQUAL-001 수락 기준

## Oracle Acceptance Notes

각 Must 시나리오는 입력 quality, role/tier, declared worker effort, capability fixture와 정확한
model/effort/reason 문자열을 비교한다. 파일 존재나 stale 문자열 검색만으로 요구사항을 닫지 않는다.
resolver 단위 테스트와 실제 root/agent/provider consumer 테스트를 함께 실행한다.

보존 oracle의 단위는 서로 다르다. root 설정은 파싱된 assignment 우변 literal을 비교하며 template이
공백을 정규화하는 것은 허용한다. pinned provider는 Binary, Args, PaneArgs의 완전한 slice equality와
각 원소의 quoting·ordering을 비교한다.

## Catalog Fixtures

| Fixture | Structured capability |
|---|---|
| `C_FULL` | Sol=`low,medium,high,xhigh,max,ultra`; Terra=`low,medium,high,xhigh,max,ultra`; Luna=`low,medium,high,xhigh,max`; `gpt-5.5`=`low,medium,high,xhigh` |
| `C_SOL_NO_ULTRA` | Sol=`xhigh,max`; `gpt-5.5`=`xhigh` |
| `C_SOL_MEDIUM_ONLY` | Sol=`medium` |
| `C_LEGACY_ONLY` | `gpt-5.5`=`low,medium,high,xhigh` |
| `C_VALID_MISSING` | unrelated model=`medium`; requested GPT-5.6 model과 `gpt-5.5`는 없음 |
| `C_INVALID` | probe error, timeout, oversized payload, empty payload, malformed JSON을 각각 주입 |

S1과 S3~S9는 정책 자체와 fallback을 섞지 않도록 `C_FULL`을 사용한다. quality-managed supervisor/orchestra에서
declared worker effort는 `not-applicable`로 명시하고, managed agent에서는 source declared effort를
입력 tuple의 일부로 명시한다.

## Test Scenarios

### S1: 중앙 resolver가 full-support 정책 행렬과 일치한다 (REQ-001)

Given `C_FULL`과 Balanced/Ultra quality-managed supervisor·orchestra의 declared effort=`not-applicable`이 주어진다
And Balanced Opus=`max`, Sonnet=`medium|high`, Haiku=`low`, Ultra 전략 3개와 나머지 worker에 `low|medium|high|max|ultra|unknown|blank` declared effort가 주어진다
When canonical desired-profile resolver와 capability resolver를 순서대로 호출한다
Then Balanced quality-managed supervisor/orchestra는 정확히 `gpt-5.6-sol+xhigh`이다
And Ultra quality-managed supervisor/orchestra는 정확히 `gpt-5.6-sol+ultra`이다
And Balanced Opus는 declared effort와 관계없이 정확히 `gpt-5.6-sol+xhigh`이다
And Balanced Sonnet/Haiku는 각각 Terra/Luna와 normalized declared effort를 사용한다
And Ultra의 `planner`, `architect`, `security-auditor`는 정확히 `gpt-5.6-sol+max`이다
And 그 외 모든 Ultra managed 또는 unknown agent role은 정확히 `gpt-5.6-sol+xhigh`이다

### S2: orchestra quality와 effort 우선순위가 결정적이다 (REQ-002, REQ-005)

Given persistent `quality.default=balanced`와 quality-managed Codex orchestra가 주어진다
When runtime `--quality ultra --effort max`를 적용한다
Then effective quality는 `runtime --quality > quality.default > balanced` 순서로 정확히 `ultra`이다
And effective effort는 `runtime --effort > quality-derived effort` 순서로 정확히 `max`이다
And runtime override가 없으면 persistent `balanced`를 사용한다
And persistent quality도 없으면 `balanced`를 사용한다
And exact `ultra`가 아닌 custom preset은 quality-managed supervisor/orchestra에서 Balanced profile을 사용한다
And custom preset으로 persistent agent를 생성할 때는 그 preset의 role tier mapping을 유지한다

### S3: fresh supervisor가 사용자 Codex 기본 모델을 상속한다 (REQ-003)

Given `supervisor_model_policy=inherit`인 fresh Codex init이 주어진다
When `.codex/config.toml`을 생성한다
Then root는 `model`과 `model_reasoning_effort` assignment를 포함하지 않는다
And Codex는 project-local override가 없으므로 사용자 runtime 기본 모델을 상속한다
And fallback receipt는 발생하지 않는다

### S4: quality-managed Ultra supervisor가 Sol+ultra를 생성한다 (REQ-003)

Given `C_FULL`, `quality.default=ultra`, `supervisor_model_policy=quality`인 Codex init이 주어진다
When `.codex/config.toml`을 생성한다
Then root는 `model="gpt-5.6-sol"`과 `model_reasoning_effort="ultra"`를 포함한다
And fallback receipt는 발생하지 않는다

### S5: Balanced agent tier가 model과 declared effort에 매핑된다 (REQ-004)

Given `C_FULL`과 planner=Opus/`max`, executor=Sonnet/`medium`, reviewer=Sonnet/`high`, synthetic=Haiku/`low`가 주어진다
When Balanced Codex agent 정의를 렌더한다
Then planner는 정확히 `gpt-5.6-sol+xhigh`이다
And executor는 정확히 `gpt-5.6-terra+medium`이다
And reviewer는 정확히 `gpt-5.6-terra+high`이다
And synthetic는 정확히 `gpt-5.6-luna+low`이다
And unknown 또는 blank declared effort인 Sonnet/Haiku는 정확히 `medium`을 사용한다
And declared `ultra`인 Sonnet/Haiku는 정확히 `max`를 사용한다

### S6: Ultra worker는 역할에 따라 Sol+max 또는 Sol+xhigh를 사용한다 (REQ-004)

Given `C_FULL`, 전략 역할 `planner|architect|security-auditor`, 그 외 managed/unknown agent role이 주어진다
And 각 역할에 source tier 및 `low|medium|high|max|ultra|unknown|blank` declared effort가 주어진다
When Ultra config로 모든 managed agent 정의를 렌더한다
Then 모든 agent는 정확히 `gpt-5.6-sol`을 사용한다
And 전략 역할 3개만 정확히 `max`를 사용한다
And 나머지 모든 managed/unknown agent role은 정확히 `xhigh`를 사용한다
And 어떤 managed agent도 `ultra` effort를 생성하지 않는다

### S7: native subagent와 team이 같은 generated agent 정의를 사용한다 (REQ-004)

Given `C_FULL`과 generated agent별 source tier 및 source declared effort가 주어진다
When 기본 `spawn_agent` 경로와 `--team` 안내 surface를 검사한다
Then 두 경로는 같은 role 이름과 `.codex/agents/*.toml` 정의를 참조한다
And 각 정의의 model과 effort는 S5 또는 S6의 tuple과 정확히 일치한다

### S8: canonical orchestra가 quality에 맞게 해석된다 (REQ-005)

Given `C_FULL`, declared effort=`not-applicable`, `model_policy: quality` Codex provider가 주어진다
When Balanced와 Ultra quality에서 provider를 각각 해석한다
Then Balanced subprocess Args와 interactive PaneArgs는 모두 `gpt-5.6-sol+xhigh`를 포함한다
And Ultra subprocess Args와 interactive PaneArgs는 모두 `gpt-5.6-sol+ultra`를 포함한다
And PaneArgs는 subprocess 전용 `exec`를 선두에 포함하지 않는다

### S9: runtime override는 quality-managed orchestra에만 즉시 적용된다 (REQ-002, REQ-005)

Given `C_FULL`, persistent `quality.default=balanced`, declared effort=`not-applicable`이 주어진다
And 같은 입력에 quality-managed provider, pinned provider, 이미 생성된 agent 파일이 주어진다
When 일반 orchestra, `orchestra run`, structured SPEC review를 `--quality ultra --effort max`로 실행한다
Then 세 경로의 quality-managed Args와 PaneArgs는 모두 정확히 `gpt-5.6-sol+max`를 사용한다
And pinned provider의 Binary, Args, PaneArgs는 입력 slice와 정확히 같다
And generated agent 파일과 disk `autopus.yaml`은 변경되지 않는다

### S10: 기존 root 설정의 소유권을 키 단위로 판별한다 (REQ-006)

Given 기존 root assignment의 우변 literal이 `model='custom/model' # keep`, `model_reasoning_effort='ultra' # keep`이다
And supervisor 정책이 없는 legacy manifest에는 markerless `gpt-5.5+medium` 사용자 tuple이 기록되어 있다
And 명시적 quality-managed config에는 model과 무관한 `approval_policy` checksum drift가 있다
When Balanced와 Ultra update를 실행하고 같은 설정으로 한 번 더 update한다
Then 두 assignment에서 파싱한 우변 literal은 각각 `'custom/model' # keep`, `'ultra' # keep`로 정확히 유지된다
And 첫 update가 남긴 키 목록 마커는 실제 사용자 소유 assignment만 다음 manifest 갱신 뒤에도 보존한다
And legacy `gpt-5.5+medium`은 명시적 supervisor 정책을 선택하기 전까지 보존된다
And 비모델 checksum drift는 생성 model/effort를 사용자 소유로 승격하지 않아 Ultra tuple로 갱신된다
And template이 assignment 주변 공백을 정규화하는 것은 root 보존 실패로 판정하지 않는다
And managed agent 파일은 persistent quality에 맞게 갱신된다

### S11: pinned provider의 전체 argv slice가 보존된다 (REQ-006)

Given `model_policy: pinned` Codex provider에 custom Binary, model, effort `ultra`, 추가 flag, 임의 ordering, `--` suffix가 있다
When migration, runtime quality/effort overlay, capability resolution을 실행한다
Then Binary는 입력과 정확히 같다
And Args와 PaneArgs의 모든 원소, ordering, quoting, `--` suffix는 입력 slice와 정확히 같다
And catalog probe를 호출하지 않는다

### S12: legacy canonical provider만 quality policy로 이행한다 (REQ-005, REQ-006)

Given marker가 없고 Args와 PaneArgs가 exact historical `gpt-5.5+xhigh` canonical tuple인 provider가 주어진다
And extra, reordered, pane-only, custom Binary 중 하나를 가진 near-match provider가 각각 주어진다
When config migration을 실행한다
Then exact tuple만 `model_policy: quality`가 되고 persistent quality profile로 해석된다
And 모든 near-match provider는 `model_policy: pinned`이 되며 Binary, Args, PaneArgs가 변하지 않는다

### S13: 같은 GPT-5.6 모델에서 effort를 먼저 낮춘다 (REQ-007)

Given `C_SOL_NO_ULTRA`가 주어진다
When `gpt-5.6-sol+ultra`를 resolve한다
Then 결과는 정확히 `gpt-5.6-sol+max`이다
And fallback reason은 정확히 `effort_unavailable`이다

### S14: 같은 모델에 더 낮거나 같은 effort가 없으면 effort만 생략한다 (REQ-007)

Given `C_SOL_MEDIUM_ONLY`가 주어진다
When `gpt-5.6-sol+low`를 resolve한다
Then effective model은 정확히 `gpt-5.6-sol`이다
And effective effort는 비어 있다
And fallback reason은 정확히 `runtime_default`이다

### S15: GPT-5.6 모델 부재 시 compatible legacy profile을 사용한다 (REQ-008)

Given `C_LEGACY_ONLY`가 주어진다
When Ultra quality-managed supervisor 또는 orchestra의 `gpt-5.6-sol+ultra`를 resolve한다
Then 결과는 정확히 `gpt-5.5+xhigh`이다
And legacy effort는 catalog가 `max`나 `ultra`를 광고해도 `xhigh`를 초과하지 않는다
And fallback reason은 정확히 `model_unavailable`이다

### S16: unavailable 또는 malformed catalog는 catalog_unknown으로 분류한다 (REQ-008)

Given `C_INVALID`의 probe error, timeout, oversized, empty, malformed JSON 사례가 각각 주어진다
When 각 사례에서 `gpt-5.6-sol+ultra`를 resolve한다
Then 각 결과는 정확히 `gpt-5.5+xhigh`이다
And fallback reason은 정확히 `catalog_unknown`이다
And receipt는 `requested`, `selected`, `reason`을 포함한다
And adapter는 동일 resolution receipt를 정확히 한 번 출력한다

### S17: valid-but-missing catalog는 runtime_default로 분류한다 (REQ-008)

Given 구조적으로 유효한 `C_VALID_MISSING`이 주어진다
When GPT-5.6 requested profile을 resolve한다
Then effective model과 effort는 모두 비어 있다
And fallback reason은 정확히 `runtime_default`이다
And root와 agent는 managed model/effort assignment를 생략한다
And quality-managed Args와 PaneArgs는 managed model/effort option만 제거한다

### S18: 모든 capability consumer가 같은 resolution을 투영한다 (REQ-001, REQ-003~REQ-008)

Given Ultra, quality-managed root declared effort=`not-applicable`, agent=executor/Sonnet/`medium`, quality-managed provider가 주어진다
When `C_FULL`, `C_SOL_NO_ULTRA`, `C_VALID_MISSING`을 동일 resolver에 각각 적용한다
Then `C_FULL`에서 root=`Sol+ultra`, agent=`Sol+xhigh`, Args/PaneArgs=`Sol+ultra`이다
And `C_SOL_NO_ULTRA`에서 root=`Sol+max`, agent=`Sol+xhigh`, Args/PaneArgs=`Sol+max`이다
And `C_VALID_MISSING`에서 root와 agent assignment가 없고 Args와 PaneArgs에도 managed model/effort option이 없다
And `C_FULL` reason은 모두 `supported`이다
And `C_SOL_NO_ULTRA` reason은 root/Args/PaneArgs에서 `effort_unavailable`, agent에서 `supported`이다
And `C_VALID_MISSING` reason은 모두 `runtime_default`이다
And 네 consumer의 selected tuple과 reason은 각각의 `CodexProfileResolution`과 정확히 같다

### S19: per-run worker 한계와 적용 시점이 정확히 문서화된다 (REQ-009)

Given 이미 custom agent 파일을 로드한 Codex 세션과 runtime quality/effort override가 주어진다
When generated Codex pipeline guidance를 검사한다
Then 안내는 현재 loaded agent model이나 effort를 바꾼다고 주장하지 않는다
And persistent worker mode 변경에는 `auto quality <mode> --apply`와 새 Codex 세션이 필요하다고 명시한다
And per-run override는 quality-managed orchestra에만 즉시 적용된다고 명시한다
And `inherit` supervisor는 사용자 Codex runtime 기본값을 유지하고 quality-managed supervisor만 Sol 프로필을 사용한다고 명시한다
And user-owned root model/effort assignment가 supervisor quality policy보다 우선해 보존된다고 명시한다

### S20: 다른 플랫폼 정책이 변하지 않는다 (REQ-010)

Given 기존 Claude route/team과 OpenCode adapter golden fixture가 주어진다
When 전체 생성 및 회귀 테스트를 실행한다
Then Claude model/effort 결과는 기존 기대값과 같다
And OpenCode는 `openai/gpt-5.4`, configured override, `--variant` 계약을 유지한다

## Scenario Evidence Map

| Scenarios | Primary automated evidence |
|---|---|
| S1, S2 | `TestQualityConfCodexSupervisorProfile`, `TestQualityConfCodexOrchestraProfile`, `TestQualityConfCodexAgentProfile`, `TestBuildProviderConfigsForRuntime_CodexQualityMatrix` |
| S3, S4 | `TestDefaultFullConfigInheritsCodexSupervisorModel`, `TestLoadLegacyYAMLMissingSupervisorPolicyKeepsQualityManagedProfile`, `TestGenerateConfig_DefaultSupervisorPolicyInheritsCodexRuntimeModel`, `TestGenerateConfig_UsesUltraQualityProfile`, `TestInitCmd_QualityBalancedInheritsCodexSupervisorAndSetsManagedAgents`, `TestInitCmd_QualityUltraInheritsCodexSupervisorAndSetsManagedAgents`, `TestQualitySupervisorCmdPersistsInheritPolicy`, `TestQualitySupervisorApplyUpdatesActualCodexRootAndAgents`, `TestQualityCmdApplyUpdatesConfiguredPlatforms`, `TestQualityCmdApplyFailureKeepsDesiredDefault` |
| S5, S6, S7 | `TestTransformAgentForCodex_RendersQualityAwareProfiles`, `TestGenerateAgents_BalancedQualityUsesRoleEffort`, `TestGenerateAgents_UltraQualityUsesSelectiveSolEffort`, template parity tests |
| S8, S9 | `TestCodexProviderEntryForQuality`, `TestLoadHarnessConfigForDir_CodexRuntimeOverridesAreEphemeral`, `TestLoadHarnessConfigForDir_RuntimeBalancedOverridesPersistentUltra`, `TestRunOrchestraCommand_AppliesRuntimeCodexQualityAndEffort`, `TestRunSubprocessPipeline_AppliesRuntimeCodexQualityAndEffort`, `TestBuildReviewProvidersWithConfig_UsesRuntimeCodexQualityProfile` |
| S10 | `TestGenerateConfig_PreservesUserModelValueLiteral`, `TestGenerateConfig_PreservesQuotedUserModelKey`, `TestGenerateConfig_IgnoresModelAndMarkerInsideMultilineString`, `TestUpdate_PreservesUserCodexModelSettings`, `TestUpdate_PreservesUserConfiguredMediumEffortWhenQualityBecomesUltra`, `TestUpdate_LegacyManagedManifestPreservesAmbiguousUserTuple`, `TestUpdate_UnrelatedConfigEditDoesNotFreezeManagedModel`, `TestUpdate_UserMarkerPreservesOnlyNamedCodexSetting`, `TestUpdate_InheritPolicyRemovesUnmodifiedManagedRootModel`, `TestUpdate_RefreshesHistoricalManagedLegacyProfile` |
| S11, S12 | `TestMigrateOrchestraConfig_ExplicitPinnedCodexRemainsByteForByte`, `TestMigrateOrchestraConfig_MarksExactHistoricalCodexDefaultsQualityManaged`, near-match and empty-provider migration tests, `TestApplyCodexProviderProfilePreservesTerminatorSuffix` |
| S13, S14, S15 | `TestResolveCodexProfile`, `TestGenerateConfig_CatalogDowngradesEffortOnSameModel`, `TestGenerateConfig_CatalogFallsBackToLegacyModel`, `TestResolveCodexProviderCapabilities_NoLowerEffortKeepsModel`, `TestResolveCodexProviderCapabilities_MissingModelUsesLegacy` |
| S16, S17 | `TestResolveCodexProfileCatalogUnknown`, catalog bounds tests, `TestGenerateConfig_CatalogUnknownUsesLegacyModel`, `TestGenerateAgents_AppliesCatalogFallbackProfiles`, `TestResolveCodexProviderCapabilities_UnknownCatalogUsesLegacy`, `TestResolveCodexProviderCapabilities_OversizedCatalogUsesLegacy`, `TestResolveCodexProviderCapabilities_OmitsOverridesForRuntimeDefault` |
| S18 | `TestCodexCapabilityMatrixProjectsEveryConsumer`가 같은 `C_FULL`, `C_SOL_NO_ULTRA`, `C_VALID_MISSING` fixture를 root, agent, Args, PaneArgs에 함께 투영하고 selected tuple/reason을 비교 |
| S19 | `TestCodexQualityGuidanceDocumentsLoadedAgentBoundary`가 adaptive-quality와 agent-pipeline의 canonical source, Codex template, Gemini template에서 inherit/quality-managed 경계, user-owned 보존, stale 무조건 supervisor 문구 부재를 함께 검증 |
| S20 | `go test ./pkg/adapter/opencode ./templates`와 전체 regression suite |

## Verification Commands

```bash
go run ./cmd/auto spec validate .autopus/specs/SPEC-CODEXQUAL-001 --strict
go test ./pkg/config ./pkg/content ./pkg/codexruntime ./pkg/adapter/codex ./pkg/adapter/opencode ./internal/cli ./templates -count=1
go test -race ./pkg/config ./pkg/content ./pkg/codexruntime ./pkg/adapter/codex ./internal/cli -count=1
go test ./... -count=1
go vet ./...
go build ./...
codex --version
codex debug models
git diff --check
git status --short
git ls-files -c -i --exclude-standard
```
