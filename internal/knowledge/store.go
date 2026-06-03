// Package knowledge 管理多租户知识库的 collection 与写入。
// 只负责连接 Milvus、collection 管理与 chunk 写入导入。
// 检索与出处召回在 agent/chat_pipeline，本包通过访问器把底层句柄暴露给它。
package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"GopherCPP/internal/config"
	"GopherCPP/pkg/constant"
)

var (
	cli      client.Client
	embedder embedding.Embedder
	cfg      config.MilvusConfig
	vecDim   int
)

type Chunk struct {
	ID         string
	Content    string
	Scope      constant.KnowledgeScope
	StudentID  string
	DocID      string
	SourceFile string
	SourceURI  string
	PageNo     int64
	ChunkIndex int64
	Metadata   map[string]any
	CreatedAt  int64
}

func Init(ctx context.Context, mc config.MilvusConfig, emb embedding.Embedder, dim int) error {
	if emb == nil {
		return fmt.Errorf("knowledge: embedder 不能为空")
	}
	if dim <= 0 {
		return fmt.Errorf("knowledge: 向量维度必须大于 0")
	}
	if mc.Collection == "" {
		mc.Collection = constant.DefaultKnowledgeCollection
	}
	c, err := client.NewClient(ctx, client.Config{
		Address:  mc.Address,
		Username: mc.Username,
		Password: mc.Password,
	})
	if err != nil {
		return fmt.Errorf("knowledge: 连接 Milvus 失败: %w", err)
	}

	cli = c
	embedder = emb
	cfg = mc
	vecDim = dim

	if err := ensureCollection(ctx); err != nil {
		_ = Close()
		return err
	}
	return nil
}

func Close() error {
	if cli == nil {
		return nil
	}
	err := cli.Close()
	cli = nil
	return err
}

// Client 返回底层 Milvus 客户端，供 chat_pipeline 构建 retriever。
func Client() client.Client { return cli }

// Embedder 返回稠密向量化器，检索查询与写入共用同一个。
func Embedder() embedding.Embedder { return embedder }

// Collection 返回知识库 collection 名。
func Collection() string { return cfg.Collection }

// VectorDim 返回稠密向量维度。
func VectorDim() int { return vecDim }

func UpsertChunks(ctx context.Context, chunks []Chunk) ([]string, error) {
	if cli == nil || embedder == nil {
		return nil, fmt.Errorf("knowledge: 未初始化")
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	texts := make([]string, 0, len(chunks))
	for i := range chunks {
		if err := normalizeChunk(&chunks[i]); err != nil {
			return nil, err
		}
		texts = append(texts, chunks[i].Content)
	}
	vectors, err := embedder.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("knowledge: 生成向量失败: %w", err)
	}
	if len(vectors) != len(chunks) {
		return nil, fmt.Errorf("knowledge: 向量数量不匹配")
	}

	ids := make([]string, 0, len(chunks))
	contents := make([]string, 0, len(chunks))
	scopes := make([]string, 0, len(chunks))
	studentIDs := make([]string, 0, len(chunks))
	docIDs := make([]string, 0, len(chunks))
	sourceFiles := make([]string, 0, len(chunks))
	sourceURIs := make([]string, 0, len(chunks))
	pageNos := make([]int64, 0, len(chunks))
	chunkIndexes := make([]int64, 0, len(chunks))
	createdAts := make([]int64, 0, len(chunks))
	metadata := make([][]byte, 0, len(chunks))
	floatVectors := make([][]float32, 0, len(chunks))

	for i, chunk := range chunks {
		vec, err := toFloat32Vector(vectors[i])
		if err != nil {
			return nil, err
		}
		meta, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return nil, fmt.Errorf("knowledge: metadata 编码失败: %w", err)
		}

		ids = append(ids, chunk.ID)
		contents = append(contents, chunk.Content)
		scopes = append(scopes, string(chunk.Scope))
		studentIDs = append(studentIDs, chunk.StudentID)
		docIDs = append(docIDs, chunk.DocID)
		sourceFiles = append(sourceFiles, chunk.SourceFile)
		sourceURIs = append(sourceURIs, chunk.SourceURI)
		pageNos = append(pageNos, chunk.PageNo)
		chunkIndexes = append(chunkIndexes, chunk.ChunkIndex)
		createdAts = append(createdAts, chunk.CreatedAt)
		metadata = append(metadata, meta)
		floatVectors = append(floatVectors, vec)
	}

	_, err = cli.Upsert(ctx, cfg.Collection, "",
		entity.NewColumnVarChar(constant.MilvusFieldID, ids),
		entity.NewColumnVarChar(constant.MilvusFieldContent, contents),
		entity.NewColumnVarChar(constant.MilvusFieldKnowledgeScope, scopes),
		entity.NewColumnVarChar(constant.MilvusFieldStudentID, studentIDs),
		entity.NewColumnVarChar(constant.MilvusFieldDocID, docIDs),
		entity.NewColumnVarChar(constant.MilvusFieldSourceFile, sourceFiles),
		entity.NewColumnVarChar(constant.MilvusFieldSourceURI, sourceURIs),
		entity.NewColumnInt64(constant.MilvusFieldPageNo, pageNos),
		entity.NewColumnInt64(constant.MilvusFieldChunkIndex, chunkIndexes),
		entity.NewColumnInt64(constant.MilvusFieldCreatedAt, createdAts),
		entity.NewColumnJSONBytes(constant.MilvusFieldMetadata, metadata),
		entity.NewColumnFloatVector(constant.MilvusFieldVector, vecDim, floatVectors),
	)
	if err != nil {
		return nil, fmt.Errorf("knowledge: 写入 Milvus 失败: %w", err)
	}
	return ids, nil
}

func ensureCollection(ctx context.Context) error {
	has, err := cli.HasCollection(ctx, cfg.Collection)
	if err != nil {
		return fmt.Errorf("knowledge: 检查 collection 失败: %w", err)
	}
	if !has {
		if err := createCollection(ctx); err != nil {
			return err
		}
	} else if err := validateCollection(ctx); err != nil {
		return err
	}
	if err := ensureIndex(ctx); err != nil {
		return err
	}
	return ensureLoaded(ctx)
}

func createCollection(ctx context.Context) error {
	s := entity.NewSchema().
		WithName(cfg.Collection).
		WithDescription("GopherCPP knowledge chunks").
		WithAutoID(false).
		WithField(entity.NewField().WithName(constant.MilvusFieldID).WithDataType(entity.FieldTypeVarChar).WithIsPrimaryKey(true).WithMaxLength(128)).
		WithField(entity.NewField().WithName(constant.MilvusFieldContent).WithDataType(entity.FieldTypeVarChar).WithMaxLength(65535)).
		WithField(entity.NewField().WithName(constant.MilvusFieldKnowledgeScope).WithDataType(entity.FieldTypeVarChar).WithMaxLength(16)).
		WithField(entity.NewField().WithName(constant.MilvusFieldStudentID).WithDataType(entity.FieldTypeVarChar).WithMaxLength(128)).
		WithField(entity.NewField().WithName(constant.MilvusFieldDocID).WithDataType(entity.FieldTypeVarChar).WithMaxLength(128)).
		WithField(entity.NewField().WithName(constant.MilvusFieldSourceFile).WithDataType(entity.FieldTypeVarChar).WithMaxLength(1024)).
		WithField(entity.NewField().WithName(constant.MilvusFieldSourceURI).WithDataType(entity.FieldTypeVarChar).WithMaxLength(2048)).
		WithField(entity.NewField().WithName(constant.MilvusFieldPageNo).WithDataType(entity.FieldTypeInt64)).
		WithField(entity.NewField().WithName(constant.MilvusFieldChunkIndex).WithDataType(entity.FieldTypeInt64)).
		WithField(entity.NewField().WithName(constant.MilvusFieldCreatedAt).WithDataType(entity.FieldTypeInt64)).
		WithField(entity.NewField().WithName(constant.MilvusFieldMetadata).WithDataType(entity.FieldTypeJSON)).
		WithField(entity.NewField().WithName(constant.MilvusFieldVector).WithDataType(entity.FieldTypeFloatVector).WithDim(int64(vecDim)))

	if err := cli.CreateCollection(ctx, s, 2); err != nil {
		return fmt.Errorf("knowledge: 创建 collection 失败: %w", err)
	}
	return nil
}

func validateCollection(ctx context.Context) error {
	desc, err := cli.DescribeCollection(ctx, cfg.Collection)
	if err != nil {
		return fmt.Errorf("knowledge: 读取 collection schema 失败: %w", err)
	}
	fields := map[string]*entity.Field{}
	for _, field := range desc.Schema.Fields {
		fields[field.Name] = field
	}
	required := map[string]entity.FieldType{
		constant.MilvusFieldID:             entity.FieldTypeVarChar,
		constant.MilvusFieldContent:        entity.FieldTypeVarChar,
		constant.MilvusFieldKnowledgeScope: entity.FieldTypeVarChar,
		constant.MilvusFieldStudentID:      entity.FieldTypeVarChar,
		constant.MilvusFieldDocID:          entity.FieldTypeVarChar,
		constant.MilvusFieldSourceFile:     entity.FieldTypeVarChar,
		constant.MilvusFieldSourceURI:      entity.FieldTypeVarChar,
		constant.MilvusFieldPageNo:         entity.FieldTypeInt64,
		constant.MilvusFieldChunkIndex:     entity.FieldTypeInt64,
		constant.MilvusFieldCreatedAt:      entity.FieldTypeInt64,
		constant.MilvusFieldMetadata:       entity.FieldTypeJSON,
		constant.MilvusFieldVector:         entity.FieldTypeFloatVector,
	}
	for name, typ := range required {
		field, ok := fields[name]
		if !ok {
			return fmt.Errorf("knowledge: collection 缺少字段 %s", name)
		}
		if field.DataType != typ {
			return fmt.Errorf("knowledge: 字段 %s 类型不匹配", name)
		}
	}
	gotDim, err := strconv.Atoi(fields[constant.MilvusFieldVector].TypeParams[entity.TypeParamDim])
	if err != nil || gotDim != vecDim {
		return fmt.Errorf("knowledge: 向量维度不匹配，期望 %d", vecDim)
	}
	return nil
}

func ensureIndex(ctx context.Context) error {
	indexes, err := cli.DescribeIndex(ctx, cfg.Collection, constant.MilvusFieldVector)
	if err == nil && len(indexes) > 0 {
		return nil
	}
	idx, err := entity.NewIndexAUTOINDEX(entity.COSINE)
	if err != nil {
		return fmt.Errorf("knowledge: 创建索引参数失败: %w", err)
	}
	if err := cli.CreateIndex(ctx, cfg.Collection, constant.MilvusFieldVector, idx, false); err != nil {
		return fmt.Errorf("knowledge: 创建向量索引失败: %w", err)
	}
	return nil
}

func ensureLoaded(ctx context.Context) error {
	state, err := cli.GetLoadState(ctx, cfg.Collection, nil)
	if err == nil && state == entity.LoadStateLoaded {
		return nil
	}
	if err := cli.LoadCollection(ctx, cfg.Collection, false); err != nil {
		return fmt.Errorf("knowledge: 加载 collection 失败: %w", err)
	}
	return nil
}

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
	if chunk.Scope == constant.KnowledgeScopePrivate && strings.TrimSpace(chunk.StudentID) == "" {
		return fmt.Errorf("knowledge: 私有知识必须有 studentID")
	}
	if chunk.Scope == constant.KnowledgeScopePublic {
		chunk.StudentID = ""
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

func toFloat32Vector(vector []float64) ([]float32, error) {
	if len(vector) != vecDim {
		return nil, fmt.Errorf("knowledge: 向量维度不匹配，期望 %d 实际 %d", vecDim, len(vector))
	}
	out := make([]float32, 0, len(vector))
	for _, v := range vector {
		out = append(out, float32(v))
	}
	return out, nil
}

func chunkID(chunk Chunk) string {
	raw := strings.Join([]string{
		string(chunk.Scope),
		chunk.StudentID,
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
