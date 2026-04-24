// Package main 提供嵌入前端静态资源的变量。
// go:embed 要求路径相对于当前 .go 文件所在目录，因此此文件放在项目根目录。
package static

import "embed"

//go:embed all:dist
var FrontendFS embed.FS
