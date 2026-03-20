// Package adapter는 PlatformAdapter 인터페이스와 공용 타입을 정의한다.
package adapter

import (
	"context"

	"github.com/insajin/autopus-adk/pkg/config"
)

// PlatformAdapter는 코딩 CLI 플랫폼별 어댑터 인터페이스이다.
type PlatformAdapter interface {
	// Name은 어댑터 이름을 반환한다 (claude-code, codex, gemini-cli 등).
	Name() string
	// Version은 어댑터 버전을 반환한다.
	Version() string
	// CLIBinary는 CLI 실행 파일명을 반환한다.
	CLIBinary() string
	// Detect는 해당 코딩 CLI의 설치 여부를 감지한다.
	Detect(ctx context.Context) (bool, error)
	// Generate는 하네스 설정에 기반하여 플랫폼 파일을 생성한다.
	Generate(ctx context.Context, cfg *config.HarnessConfig) (*PlatformFiles, error)
	// Update는 기존 파일을 업데이트한다 (사용자 수정 보존).
	Update(ctx context.Context, cfg *config.HarnessConfig) (*PlatformFiles, error)
	// Validate는 설치된 파일의 유효성을 검증한다.
	Validate(ctx context.Context) ([]ValidationError, error)
	// Clean은 어댑터가 생성한 파일을 제거한다.
	Clean(ctx context.Context) error
	// SupportsHooks는 코딩 CLI 훅 지원 여부를 반환한다.
	SupportsHooks() bool
	// InstallHooks는 코딩 CLI 훅을 설치한다.
	InstallHooks(ctx context.Context, hooks []HookConfig) error
}

// PlatformFiles는 어댑터가 생성한 파일 목록이다.
type PlatformFiles struct {
	Files    []FileMapping `json:"files"`
	Checksum string        `json:"checksum"`
}

// FileMapping은 단일 파일 매핑이다.
type FileMapping struct {
	SourceTemplate  string          `json:"source_template"`
	TargetPath      string          `json:"target_path"`
	OverwritePolicy OverwritePolicy `json:"overwrite_policy"`
	Checksum        string          `json:"checksum"`
	Content         []byte          `json:"-"`
}

// OverwritePolicy는 파일 덮어쓰기 정책이다.
type OverwritePolicy string

const (
	OverwriteAlways OverwritePolicy = "always"
	OverwriteNever  OverwritePolicy = "never"
	OverwriteMarker OverwritePolicy = "marker" // AUTOPUS:BEGIN/END 마커 섹션만 업데이트
	OverwriteMerge  OverwritePolicy = "merge"  // 기존 내용에 머지 (예: .mcp.json)
)

// ValidationError는 검증 에러이다.
type ValidationError struct {
	File    string `json:"file"`
	Message string `json:"message"`
	Level   string `json:"level"` // error, warning
}

// HookConfig는 훅 설정이다.
type HookConfig struct {
	Event   string `json:"event"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}
