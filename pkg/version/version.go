// Package version은 빌드 시 ldflags로 주입되는 버전 정보를 제공한다.
package version

import "fmt"

// 빌드 시 ldflags로 주입
var (
	version = "0.6.0"
	commit  = "none"
	date    = "unknown"
)

// Version은 빌드 버전을 반환한다.
func Version() string { return version }

// Commit은 빌드 커밋 해시를 반환한다.
func Commit() string { return commit }

// Date은 빌드 날짜를 반환한다.
func Date() string { return date }

// String은 전체 버전 문자열을 반환한다.
func String() string {
	return fmt.Sprintf("auto %s (commit: %s, built: %s)", version, commit, date)
}
