package models

import "time"

type UploadReq struct {
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
	UserID      string `json:"user_id"`
}
type UploadResp struct {
	DocId     string            `json:"doc_id"`
	UploadURL string            `json:"upload_url"`
	FileKey   string            `json:"file_key"`
	Fields    map[string]string `json:"fields,omitempty"`
	Expires   time.Time         `json:"expires"`
	Provider  string            `json:"provider"` // "minio" or "s3"
}

type ConfirmUploadReq struct {
	DocId    string `json:"doc_id"`
	ApiKey   string `json:"api_key"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	RagMode  string `json:"rag_mode"`
}
type ConfirmUploadResp struct {
	Message string `json:"message"`
	DocId   string `json:"doc_id"`
	Status  string `json:"status"`
}
