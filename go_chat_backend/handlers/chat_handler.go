package handlers

import (
	"github.com/gofiber/fiber/v2"
	"go_chat_backend/models"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/services"
	"time"
)

type ChatHandler struct {
	cacheService *services.CacheService
}

func NewChatHandler(cacheService *services.CacheService) *ChatHandler {
	return &ChatHandler{cacheService: cacheService}
}
func (h *ChatHandler) AskQuestions(c *fiber.Ctx) error {
	docID := c.Params("doc_id")

	// decompose message
	var req models.AskQuestionRequest
	var err error
	if err = c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	// retrieve through parent
	var history []models.ChatNode
	var sectionID string
	if req.ParentID != "" {
		// try to get from the cache
		cacheData, ok := h.cacheService.GetCacheHistory(req.ParentID)
		if ok {
			if cachedHistory, ok := cacheData.([]models.ChatNode); ok {
				history = cachedHistory
			} else {
				ok = false
			}
		}
		if !ok {
			// find history in db if you couldn't find in cache
			history, err = services.GetChatHistory(docID, req.ParentID)
			if err != nil {
				return fiber.NewError(fiber.StatusInternalServerError, err.Error())
			}
			if len(history) == 1 {
				sectionID, err = services.ExtractChapter(docID, req.Question)
				if err != nil {
					return fiber.NewError(fiber.StatusInternalServerError, err.Error())
				}
			} else if len(history) > 1 {
				sectionID = history[1].SectionID
			}
		}
	}
	// built prompt
	prompt := services.BuildPrompt(history, req.Question, sectionID)
	// get answer
	answer, err := services.CallLLM(prompt, req.Model, req.ModelVersion)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "LLM call failed")
	}
	// save chat node
	nodeID, node, err := services.SaveChatNode(req.Question, answer, docID, req.ParentID, sectionID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to save chat")
	}
	// concurrently set chat history in cache
	go func(history []models.ChatNode, nodeID string, node models.ChatNode) {
		newHistory := append(history, node)
		if err := h.cacheService.SetCacheHistory(nodeID, newHistory, 2*time.Hour); err != nil {
			logging.Logger.Error("fail SetCacheHistory", err)
		}
		if err := h.cacheService.DelCacheHistory(req.ParentID); err != nil {
			logging.Logger.Error("fail DelCacheHistory", err)
		}
	}(history, nodeID, node)
	return c.JSON(fiber.Map{
		"answer":   answer,
		"node_id":  nodeID,
		"question": req.Question,
	})
}
func GetTree(c *fiber.Ctx) (err error) {
	docID := c.Params("doc_id")
	document, err := services.GetDocument(docID)
	if err != nil {
		return err
	}
	root := document.Root

	tree, err := services.BuildTree(docID, root)
	if err != nil {
		return err
	}
	return c.JSON(tree)
}
