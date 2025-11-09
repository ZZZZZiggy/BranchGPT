package models

import "time"

type ChatNode struct {
	ID        string `gorm:"primaryKey"`
	ParentID  string
	FileID    string
	Question  string
	Answer    string
	CreatedAt time.Time
}

type ChatTreeNode struct {
	ID       string          `json:"id"`
	Question string          `json:"question"`
	Answer   string          `json:"answer"`
	Children []*ChatTreeNode `json:"children"`
}
type ChatReq struct {
	FileID    string
	UserID    string // 用户 ID，用于获取 LLM 配置
	Question  string
	Section   string
	ParentID  string
	Model     string
	CreatedAt time.Time
	Provider  string
	APIKey    string
}

type ChatRes struct {
	ID       string        `json:"id"`
	Answer   string        `json:"answer"`
	Question string        `json:"question"`
	Tree     *ChatTreeNode `json:"tree"`
}
