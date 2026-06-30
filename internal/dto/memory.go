package dto

type MemoryRequest struct {
	Type          string   `json:"type"`
	Title         string   `json:"title" binding:"required"`
	Content       string   `json:"content" binding:"required"`
	Tags          []string `json:"tags"`
	SourcePaperID string   `json:"source_paper_id"`
	Pinned        bool     `json:"pinned"`
}

type MemorySearchRequest struct {
	Q    string `json:"q"`
	Type string `json:"type"`
	Tag  string `json:"tag"`
}
