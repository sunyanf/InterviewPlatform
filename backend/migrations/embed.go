// Package migrations 通过 go:embed 携带全部 SQL 迁移文件，
// 使部署产物为单个二进制（容器内无需挂载 migrations 目录）。
package migrations

import "embed"

// FS 编译期内嵌的 migrations 目录（*.up.sql / *.down.sql）
//
//go:embed *.sql
var FS embed.FS
