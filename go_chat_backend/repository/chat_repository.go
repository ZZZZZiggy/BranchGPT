package repository

import (
	"context"
	"go_chat_backend/models"
	"go_chat_backend/pkg/logging"
	"gorm.io/gorm"
)

type chatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) ChatRepository {
	return &chatRepository{db: db}
}
func (r *chatRepository) Create(ctx context.Context, node *models.ChatNode) error {
	return r.db.WithContext(ctx).Create(node).Error
}
func (r *chatRepository) GetChatHistory(ctx context.Context, fileID string, nodeID string) ([]*models.ChatNode, error) {
	var res []*models.ChatNode
	currID := nodeID
	for currID != "" {
		var node models.ChatNode
		if err := r.db.WithContext(ctx).Where("file_id = ? AND id = ?", fileID, currID).First(&node).Error; err != nil {
			logging.Logger.Error("fail GetChatHistory", err)
			return nil, err
		}
		res = append([]*models.ChatNode{&node}, res...)
		currID = node.ParentID
	}
	return res, nil
}
func (r *chatRepository) GetChatChildren(ctx context.Context, fileID string, nodeID string) ([]*models.ChatNode, error) {
	var res []*models.ChatNode
	err := r.db.WithContext(ctx).Where("file_id = ? AND parent_id = ?", fileID, nodeID).Find(&res).Error
	if err != nil {
		logging.Logger.Error("fail GetChatChildren", err)
		return nil, err
	}
	return res, nil
}
func (r *chatRepository) GetNodeByID(ctx context.Context, nodeID string, fileID string) (*models.ChatNode, error) {
	var res models.ChatNode
	err := r.db.WithContext(ctx).Where("id = ? AND file_id = ?", nodeID, fileID).First(&res).Error
	if err != nil {
		logging.Logger.Error("fail GetNodeByID", err)
		return nil, err
	}
	return &res, nil
}
