package web

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed public
var files embed.FS

func Public() fs.FS {
	public, err := fs.Sub(files, "public")
	if err != nil {
		panic(err)
	}
	return public
}

func IndexHTML() []byte {
	b, err := files.ReadFile("public/index.html")
	if err != nil {
		return missingHTML("GopherPaper 科研文献智能助手")
	}
	return b
}

// ReaderHTML 返回精读页入口,/reader 路由直出,前端按 ?id 渲染 PDF。
func ReaderHTML() []byte {
	b, err := files.ReadFile("public/reader.html")
	if err != nil {
		return missingHTML("精读 · GopherPaper")
	}
	return b
}

func HasFrontendBuild() bool {
	_, err := files.ReadFile("public/index.html")
	return err == nil
}

func missingHTML(title string) []byte {
	return []byte(fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>%s</title>
  </head>
  <body>
    <div id="root">前端构建产物不存在,请先在 web 目录执行 npm run build,再重新编译 Go 服务。</div>
  </body>
</html>`, title))
}

func Static() fs.FS {
	static, err := fs.Sub(files, "public/static")
	if err != nil {
		panic(err)
	}
	return static
}
