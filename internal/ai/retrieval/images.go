// images.go 是图块召回的共享原语:从召回结果里挑图块、把图片本体读成带图问答载荷、
// 生成 figure:// 插图指示。chat 的单轮带图路径与 maodie 精读页问答共用,保证两处行为一致。
package retrieval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// ImagePayload 是随消息发给多模态模型的一张命中图,Format 为不带点的扩展名。
type ImagePayload struct {
	Data   []byte
	Format string
}

// ImageDocs 从召回结果里挑出图块(block_type==image),不改动入参顺序。
func ImageDocs(docs []*Doc) []*Doc {
	var out []*Doc
	for _, d := range docs {
		if MetaString(d, constant.MilvusFieldBlockType) == constant.BlockTypeImage {
			out = append(out, d)
		}
	}
	return out
}

// LoadImagePayloads 把命中图块的本地图片读成带图问答载荷,单张读失败只记日志跳过(其 caption 仍在 context)。
func LoadImagePayloads(imgDocs []*Doc) []ImagePayload {
	images := make([]ImagePayload, 0, len(imgDocs))
	for _, d := range imgDocs {
		uri := MetaString(d, constant.MilvusFieldImgURI)
		if uri == "" {
			continue
		}
		data, err := os.ReadFile(uri)
		if err != nil {
			zlog.Error("读取召回图片失败,跳过", "img_uri", uri, "err", err)
			continue
		}
		images = append(images, ImagePayload{Data: data, Format: imageFormat(uri)})
	}
	return images
}

// FigureInstruction 在有召回图时生成插图指示:让模型用 figure://文件名 占位把图插进正文对应位置,
// 文件名只能取自清单(即图块图片名),前端再把占位解析成带 token 的取图 URL。无图返回空串。
func FigureInstruction(imgDocs []*Doc) string {
	if len(imgDocs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n下面是与问题相关、已随消息提供给你的图片。只要图能直观支撑回答,就用 Markdown 图片语法 ![简短说明](figure://文件名) 把它插入到正文对应位置,并在正文里点明该图说明了什么;文件名只能用下面列出的,不要编造,确实没有相关图时才不插:\n")
	for _, d := range imgDocs {
		name := filepath.Base(MetaString(d, constant.MilvusFieldImgURI))
		if name == "" || name == "." {
			continue
		}
		fmt.Fprintf(&b, "- figure://%s : %s\n", name, summarize(d.Content, 40))
	}
	return b.String()
}

// summarize 把图块说明压成单行短摘要,作插图清单的图片标注。
func summarize(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// imageFormat 从图片路径扩展名推出模型需要的 format(不带点),无法识别回退 png。
func imageFormat(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "jpg", "jpeg", "png", "webp", "gif":
		return ext
	default:
		return "png"
	}
}
