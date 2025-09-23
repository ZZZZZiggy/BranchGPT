package models

import "time"

type AskQuestionRequest struct {
	Question     string
	ParentID     string
	Model        string
	CreatedAt    time.Time
	ModelVersion string
}
type ChatTreeNode struct {
	ID       string          `json:"id"`
	Question string          `json:"question"`
	Answer   string          `json:"answer"`
	Children []*ChatTreeNode `json:"children"`
}
