// Package arch는 프로젝트 아키텍처 분석 및 생성 기능을 제공한다.
package arch

// ArchitectureMap는 프로젝트 아키텍처 전체 구조이다.
type ArchitectureMap struct {
	Domains      []Domain     // 도메인 목록
	Layers       []Layer      // 레이어 목록
	Dependencies []Dependency // 의존성 목록
	Violations   []Violation  // 위반 목록
}

// Domain은 프로젝트 내 논리적 도메인이다.
type Domain struct {
	Name        string   // 도메인명
	Path        string   // 디렉터리 경로
	Description string   // 설명
	Packages    []string // 포함된 패키지 목록
}

// Layer는 아키텍처 레이어이다.
type Layer struct {
	Name        string   // 레이어명 (예: cmd, pkg, internal)
	Level       int      // 레이어 레벨 (높을수록 상위)
	AllowedDeps []string // 의존 가능한 레이어 목록
}

// Dependency는 패키지 간 의존 관계이다.
type Dependency struct {
	From string // 의존하는 패키지
	To   string // 의존받는 패키지
	Type string // 의존 유형 (import, require 등)
}

// Violation은 아키텍처 규칙 위반이다.
type Violation struct {
	Rule        string // 위반한 규칙명
	From        string // 위반 출처 패키지
	To          string // 위반 대상 패키지
	Message     string // 위반 메시지
	Remediation string // 수정 방법
}

// LintRule은 아키텍처 린트 규칙이다.
type LintRule struct {
	Name        string // 규칙명
	FromLayer   string // 출처 레이어
	ToLayer     string // 대상 레이어
	Allowed     bool   // 허용 여부
	Remediation string // 수정 방법
}

// ValidationError는 검증 오류이다.
type ValidationError struct {
	Field   string // 오류 필드
	Message string // 오류 메시지
}

func (v ValidationError) Error() string {
	return v.Message
}
