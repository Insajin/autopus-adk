// Package content는 빌트인 컨텐츠 파일을 Go 바이너리에 임베딩한다.
// skills/, agents/, hooks/, methodology/ 하위 파일이 포함된다.
package content

import "embed"

// FS는 임베딩된 컨텐츠 파일시스템이다.
//
//go:embed skills/*.md agents/*.md hooks/*.sh methodology/*.yaml
var FS embed.FS
