package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go_chat_backend/models"
	"io"
	"net/http"
	"os"
	"strings"
)

func BuildPrompt(history []models.ChatNode, question string, sectionID string) string {
	var builder strings.Builder
	builder.WriteString("You are an AI assistant helping the user understand a technical document.\n\n")

	if len(history) > 0 {
		//sectionID := history[len(history)-1].SectionID
		docID := history[len(history)-1].DocID
		sectionText, err := GetSection(docID, sectionID)
		if err != nil {
			fmt.Printf("Error getting section: %v\n", err)
			sectionText = ""
		}
		if sectionID != "" && sectionText != "" {
			builder.WriteString(fmt.Sprintf("The following questions are about Section %s:\n%s\n\n", sectionID, sectionText))
		}
	}

	if len(history) > 0 {
		builder.WriteString("Previous conversation:\n")
		for i, node := range history {
			builder.WriteString(fmt.Sprintf("Q%d: %s\n", i+1, node.Question))
			builder.WriteString(fmt.Sprintf("A%d: %s\n", i+1, node.Answer))
		}
	}

	builder.WriteString("\nNow answer the following question in context of the above:\n")
	builder.WriteString("Q: " + question + "\n")

	return builder.String()
}

func FilePrompt(msg string, ProcessedFile map[string]interface{}) (string, error) {
	res := msg + "\n"
	paragraphs, ok := ProcessedFile["paragraphs"].([]interface{})
	if !ok || len(paragraphs) == 0 {
		return "", fmt.Errorf("paragraphs not found or empty")
	}
	for _, paragraph := range paragraphs {
		paragraph, ok := paragraph.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("invalid data structure: paragraph not found or wrong type")
		}
		chapter, ok := paragraph["chapter"].(string)
		if !ok {
			return "", fmt.Errorf("invalid data structure: chapter not found or wrong type")
		}
		content, ok := paragraph["content"].(string)
		if !ok {
			return "", fmt.Errorf("invalid data structure: content not found or wrong type")
		}
		res += fmt.Sprintf("%s\n", chapter)
		res += fmt.Sprintf("%s\n", content)
	}
	return res, nil
}

func CallLLM(prompt string, provider string, modelName string) (string, error) {
	switch provider {
	case "OpenAI":
		return CallOpenAI(prompt, modelName)
	case "Gemini":
		return CallGemini(prompt, modelName)
	case "Claude":
		return CallClaude(prompt, modelName)
	default:
		return "", fmt.Errorf("invalid provider")
	}
}

func CallOpenAI(prompt string, modelName string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not found")
	}

	reqBody := models.ChatRequest{
		Model: modelName,
		Messages: []models.ChatMessage{
			{Role: "user", Content: "You are a helpful assistant."},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   2000,
		Temperature: 0.7,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	// request head
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Error closing response body, %v", err)
		}
	}(resp.Body)

	// get response
	var openaiResp models.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	if len(openaiResp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return openaiResp.Choices[0].Message.Content, nil

}

func CallGemini(prompt string, modelName string) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY not found")
	}

	// Gemini API 请求体结构
	type GeminiContent struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}

	type GeminiRequest struct {
		Contents []GeminiContent `json:"contents"`
	}

	type GeminiResponse struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []struct {
					Text string `json:"text"`
				}{
					{Text: prompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// 使用 gemini-1.5-flash 模型 (更快且免费) 或 gemini-1.5-pro (更强大但收费)
	//modelName := "gemini-1.5-flash" // 您也可以使用 "gemini-1.5-pro"
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", modelName, apiKey)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}(resp.Body)

	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if geminiResp.Error.Code != 0 {
		return "", fmt.Errorf("Gemini API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

func CallClaude(prompt string, modelName string) (string, error) {
	apiKey := os.Getenv("CLAUDE_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("CLAUDE_API_KEY not found")
	}

	// Claude API 请求体结构
	type ClaudeMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type ClaudeRequest struct {
		Model     string          `json:"model"`
		MaxTokens int             `json:"max_tokens"`
		Messages  []ClaudeMessage `json:"messages"`
	}

	type ClaudeResponse struct {
		Content []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		} `json:"content"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	reqBody := ClaudeRequest{
		Model:     modelName,
		MaxTokens: 2000,
		Messages: []ClaudeMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Claude API 需要特定的头部
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}(resp.Body)

	var claudeResp ClaudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if claudeResp.Error.Message != "" {
		return "", fmt.Errorf("claude API error: %s", claudeResp.Error.Message)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("no response from Claude")
	}

	return claudeResp.Content[0].Text, nil
}
