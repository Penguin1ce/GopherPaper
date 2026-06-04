package web

import (
	"embed"
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
		panic(err)
	}
	return b
}

func Static() fs.FS {
	static, err := fs.Sub(files, "public/static")
	if err != nil {
		panic(err)
	}
	return static
}
