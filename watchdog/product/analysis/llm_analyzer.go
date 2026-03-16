package analysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-toolkit/watchdog/utils"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type chatCompletionRequest struct {
	Model    string              `json:"model"`
	Messages []chatMessage       `json:"messages"`
	Stream   bool                `json:"stream"`
	Temp     float64             `json:"temperature,omitempty"`
	Meta     map[string][]string `json:"metadata,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func AnalyzeWithLLM(baseURL, apiKey, model, prompt, siftResult string) (string, error) {
	raw := strings.TrimSpace(siftResult)
	if raw == "" || raw == "No data available." {
		log.Println("[LLM] Empty or no data, returning original result")
		return raw, nil
	}

	baseURL = strings.TrimSpace(baseURL)
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)

	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	if model == "" {
		model = strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	if baseURL == "" {
		log.Println("[LLM] Missing baseURL, returning original result")
		return raw, nil
	}

	baseURL = strings.TrimRight(baseURL, "/")
	endpoint := baseURL + "/v1/chat/completions"

	reqBody := chatCompletionRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: raw},
		},
		Stream: false,
		Temp:   0.3,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[LLM] Failed to marshal request body: %v\n", err)
		return raw, err
	}

	log.Printf("[LLM] Sending request to %s with model=%s\n", endpoint, model)

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("[LLM] Failed to create HTTP request: %v\n", err)
		return raw, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := utils.DoRequestWithRetry(req)
	if err != nil {
		log.Printf("[LLM] HTTP request failed: %v\n", err)
		return raw, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[LLM] Failed to read response body: %v\n", err)
		return raw, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("[LLM] HTTP status %d: %s\n", resp.StatusCode, string(body))
		return raw, fmt.Errorf("llm http status %d: %s", resp.StatusCode, string(body))
	}

	var completion chatCompletionResponse
	if err = json.Unmarshal(body, &completion); err != nil {
		log.Printf("[LLM] Failed to unmarshal response: %v\n", err)
		return raw, err
	}
	if len(completion.Choices) == 0 {
		log.Println("[LLM] No choices in response, returning original result")
		return raw, nil
	}

	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	if content == "" {
		log.Println("[LLM] Empty content in LLM response, returning original result")
		return raw, nil
	}
	log.Printf("[LLM] LLM response: %s\n", content)
	return content, nil
}
