package models

import "time"

type DocumentEventType string

const (
	EventDocumentProcessing DocumentEventType = "processing"
	EventDocumentCompleted  DocumentEventType = "completed"
	EventDocumentFailed     DocumentEventType = "failed"
)

type ProgressInfo struct {
	ChunksReceived int32 `json:"chunks_received"`
	ChunksStored   int32 `json:"chunks_stored"`
	ChunksFailed   int32 `json:"chunks_failed"`
	TotalChunks    int32 `json:"total_chunks"`
	Percentage     int   `json:"percentage"`
}

type DocumentEvent struct {
	Type      DocumentEventType `json:"type"`
	DocID     string            `json:"doc_id"`
	UserID    string            `json:"user_id"`
	Status    string            `json:"status"`
	Message   string            `json:"message"`
	Summary   string            `json:"summary,omitempty"`
	Sections  []string          `json:"sections,omitempty"`  // ✅ 修复：改为 []string
	Progress  *ProgressInfo     `json:"progress,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}
