package handlers

import (
	"github.com/gofiber/fiber/v2"
	"go_chat_backend/models"
	"go_chat_backend/services"
)

func AskQuestions(c *fiber.Ctx) error {
	docID := c.Params("doc_id")

	// decompose message
	var req models.AskQuestionRequest
	var err error
	if err = c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	// retrieve parents till root
	var history []models.ChatNode
	var sectionID string
	if req.ParentID != "" {
		// find history
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
	prompt := services.BuildPrompt(history, req.Question, sectionID)
	answer, err := services.CallLLM(prompt, req.Model, req.ModelVersion)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "LLM call failed")
	}
	nodeID, err := services.SaveChatNode(req.Question, answer, docID, req.ParentID, sectionID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to save chat")
	}
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
