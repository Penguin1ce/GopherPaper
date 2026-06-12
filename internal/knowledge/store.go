// Package knowledge 管理多租户知识库的写入与 chunk 规整。
// 向量库连接、collection 管理、写入与检索经 trpc vectorstore(见 milvus.go),
// 本文件只留与底层 SDK 无关的 Chunk 模型与规整逻辑。
package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"GopherPaper/pkg/constant"
)

type Chunk struct {
	ID         string
	Content    string
	Scope      constant.KnowledgeScope
	OwnerID    string // 私有库归属用户，落 student_id 字段
	DocID      string
	SourceFile string
	SourceURI  string
	PageNo     int64
	ChunkIndex int64
	Metadata   map[string]any
	CreatedAt  int64
}

// normalizeChunk 校验并补全 chunk 的默认字段,生成稳定 ID。
func normalizeChunk(chunk *Chunk) error {
	chunk.Content = strings.TrimSpace(chunk.Content)
	if chunk.Content == "" {
		return fmt.Errorf("knowledge: chunk 内容不能为空")
	}
	if chunk.Scope == "" {
		chunk.Scope = constant.KnowledgeScopePrivate
	}
	if chunk.Scope != constant.KnowledgeScopePublic && chunk.Scope != constant.KnowledgeScopePrivate {
		return fmt.Errorf("knowledge: scope 不合法")
	}
	if chunk.Scope == constant.KnowledgeScopePrivate && strings.TrimSpace(chunk.OwnerID) == "" {
		return fmt.Errorf("knowledge: 私有知识必须有 ownerID")
	}
	if chunk.Scope == constant.KnowledgeScopePublic {
		chunk.OwnerID = ""
	}
	if chunk.CreatedAt == 0 {
		chunk.CreatedAt = time.Now().Unix()
	}
	if chunk.Metadata == nil {
		chunk.Metadata = map[string]any{}
	}
	if chunk.ID == "" {
		chunk.ID = chunkID(*chunk)
	}
	return nil
}

// chunkID 用稳定字段拼出内容指纹作主键,保证同一片段重复写入幂等。
func chunkID(chunk Chunk) string {
	raw := strings.Join([]string{
		string(chunk.Scope),
		chunk.OwnerID,
		chunk.DocID,
		chunk.SourceFile,
		chunk.SourceURI,
		strconv.FormatInt(chunk.PageNo, 10),
		strconv.FormatInt(chunk.ChunkIndex, 10),
		chunk.Content,
	}, "\x1f")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
