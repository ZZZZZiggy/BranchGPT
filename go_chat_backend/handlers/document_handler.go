package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go_chat_backend/services"
	"sync"
)

type DocHandler struct {
	docService *services.StorageService
}

func NewDocHandler(docService *services.StorageService) *DocHandler {
	return &DocHandler{docService: docService}
}

func (h *DocHandler) RequestUpload(c *fiber.Ctx) error {
	var req struct {
		FileName    string `json:"file_name"`
		FileSize    int64  `json:"file_size"`
		ContentType string `json:"content_type"`
		UserID      string `json:"user_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.FileSize > 50*1024*1024 {
		return c.Status(400).JSON(fiber.Map{"error": "File too large"})
	}

	if req.ContentType != "application/pdf" {
		return c.Status(400).JSON(fiber.Map{"error": "Only PDF files allowed"})
	}

	res, err := h.docService.GeneratePresignedPostUpload(
		req.FileName, req.FileSize)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate presigned URL"})
	}
	return c.JSON(res)
}
func ProcessPDF(c *fiber.Ctx) error {
	docID := uuid.New().String()
	// get FormFile from fiber
	fileHandler, err := c.FormFile("file")
	model := c.FormValue("model", "Gemini")
	modelVision := c.FormValue("model_vision")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "File required")
	}

	// python api
	ProcessedFile, err := services.CallPythonAPI(fileHandler, docID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// sync save redis and postgres
	var wg sync.WaitGroup
	errChan := make(chan error, 2)
	q1Chan := make(chan map[string]string, 1)
	sectionChan := make(chan []string, 1)
	// save to redis
	wg.Add(1)
	go func() {
		defer wg.Done()
		sections, err := services.SaveToRedis(ProcessedFile, docID)
		if err != nil {
			errChan <- err
		}
		sectionChan <- sections
	}()

	// call llm for first message
	wg.Add(1)
	go func() {
		defer wg.Done()
		msg := "Please summarize the main content of this paper and its research category"
		prompt, err := services.FilePrompt(msg, ProcessedFile)
		if err != nil {
			errChan <- err
			return
		}

		answer, err := services.CallLLM(prompt, model, modelVision)
		if err != nil {
			errChan <- err
			return
		}
		nodeID, _, err := services.SaveChatNode(msg, answer, docID, "", "")
		if err != nil {
			errChan <- err
			return
		}
		q1Chan <- map[string]string{
			"answer":  answer,
			"node_id": nodeID,
		}
	}()
	wg.Wait()
	close(errChan)
	close(q1Chan)
	close(sectionChan)
	for err := range errChan {
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
	}
	// get Q1 answer
	var q1answer string
	var q1nodeID string
	var sections []string
	if q1, ok := <-q1Chan; ok {
		q1answer = q1["answer"]
		q1nodeID = q1["node_id"]
	}
	if s, ok := <-sectionChan; ok {
		sections = s
	}
	// save to postgres
	docMeta, err := services.SaveToPostgres(ProcessedFile, docID, q1nodeID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())

	}
	return c.JSON(fiber.Map{
		"message":    "PDF processed successfully",
		"doc_id":     docID,
		"title":      docMeta.Title,
		"status":     "processed",
		"created_at": docMeta.CreatedAt,
		"q1_answer":  q1answer,
		"q1_node_id": q1nodeID,
		"sections":   sections,
	})

}
