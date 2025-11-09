package handlers

import (
	"context"
	"go_chat_backend/models"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/services"

	"github.com/gofiber/fiber/v2"
)

type DocHandler struct {
	documentService  *services.DocumentService
	grpcService      *services.GRPCService
	llmConfigService *services.LLMConfigService
}

func NewDocHandler(documentService *services.DocumentService, grpcService *services.GRPCService, llmConfigService *services.LLMConfigService) *DocHandler {
	return &DocHandler{
		documentService:  documentService,
		grpcService:      grpcService,
		llmConfigService: llmConfigService,
	}
}

func (h *DocHandler) RequestUpload(c *fiber.Ctx) error {
	var req models.UploadReq
	if err := c.BodyParser(&req); err != nil {
		logging.Logger.Error("fail RequestUpload", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	if req.FileName == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "file_name is required",
		})
	}
	if req.UserID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "user_id is required",
		})
	}
	res, err := h.documentService.RequestUpload(c.Context(), req)
	if err != nil {
		logging.Logger.Error("fail RequestUpload", "error", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to request upload", "details": err.Error()})
	}

	// return to frontend
	return c.JSON(res)
}

func (h *DocHandler) ConfirmUpload(c *fiber.Ctx) error {
	var req models.ConfirmUploadReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	ctx := context.Background()

	docInfo, err := h.documentService.GetDocumentByID(ctx, req.DocId)
	if err != nil {
		logging.Logger.Error("fail to get document info", "error", err, "docID", req.DocId)
		return c.Status(404).JSON(fiber.Map{"error": "Document not found"})
	}

	// 保存用户的 LLM 配置到缓存（30分钟有效期）
	if req.ApiKey != "" && req.Model != "" && req.Provider != "" {
		llmConfig := &services.LLMConfig{
			APIKey:   req.ApiKey,
			Model:    req.Model,
			Provider: req.Provider,
			UserID:   docInfo.UserID,
		}
		if err := h.llmConfigService.SetUserLLMConfig(ctx, docInfo.UserID, llmConfig); err != nil {
			logging.Logger.Error("fail to save LLM config", "error", err, "userID", docInfo.UserID)
		} else {
			logging.Logger.Info("LLM config saved",
				"userID", docInfo.UserID,
				"provider", req.Provider,
				"model", req.Model,
				"apiKey", services.MaskAPIKey(req.ApiKey),
			)
		}
	}

	res, err := h.documentService.ConfirmUpload(c.Context(), req)
	if err != nil {
		logging.Logger.Error("fail ConfirmUpload", "error", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to confirm upload"})
	}
	return c.JSON(res)
}

func (h *DocHandler) GetToc(c *fiber.Ctx) error {
	docID := c.Params("doc_id")
	res, err := h.documentService.GetSections(c.Context(), docID)
	if err != nil {
		logging.Logger.Error("fail GetToc", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get TOC"})
	}
	return c.JSON(res)
}
