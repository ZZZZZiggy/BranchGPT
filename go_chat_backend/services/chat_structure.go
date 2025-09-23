package services

import (
	"github.com/google/uuid"
	"go_chat_backend/models"
	"time"
)

func GetChatHistory(docID string, ID string) ([]models.ChatNode, error) {
	var result []models.ChatNode
	currID := ID
	for currID != "" {
		var node models.ChatNode
		err := DB.Where("doc_id = ? AND id = ?", docID, currID).First(&node).Error
		if err != nil {
			return nil, err
		}
		result = append([]models.ChatNode{node}, result...)

		currID = node.ParentID
	}
	return result, nil
}

func SaveChatNode(question string, answer string, docID string, parentID string, sectionID string) (string, error) {
	ID := uuid.New().String()
	nodeMeta := &models.ChatNode{
		Question:  question,
		Answer:    answer,
		DocID:     docID,
		ID:        ID,
		ParentID:  parentID,
		CreatedAt: time.Now(),
		SectionID: sectionID,
	}
	if err := DB.Create(&nodeMeta).Error; err != nil {
		return "", err
	}
	return nodeMeta.ID, nil
}

func GetChatChildren(docID string, parentID string) ([]models.ChatNode, error) {
	var children []models.ChatNode
	err := DB.Where("doc_id = ? AND parent_id = ?", docID, parentID).Find(&children).Error
	return children, err
}

func BuildTree(docID string, rootID string) (models.ChatTreeNode, error) {
	var rootNode models.ChatNode
	if err := DB.Where("id = ?", rootID).First(&rootNode).Error; err != nil {
		return models.ChatTreeNode{}, err
	}

	root := &models.ChatTreeNode{
		ID:       rootNode.ID,
		Question: rootNode.Question,
		Answer:   rootNode.Answer,
	}

	queue := []struct {
		Node   *models.ChatTreeNode
		NodeID string
	}{{root, rootID}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		children, _ := GetChatChildren(docID, curr.NodeID)
		for _, child := range children {
			childTree := &models.ChatTreeNode{
				ID:       child.ID,
				Question: child.Question,
				Answer:   child.Answer,
			}
			curr.Node.Children = append(curr.Node.Children, childTree)
			queue = append(queue, struct {
				Node   *models.ChatTreeNode
				NodeID string
			}{childTree, child.ID})
		}
	}

	return *root, nil
}
