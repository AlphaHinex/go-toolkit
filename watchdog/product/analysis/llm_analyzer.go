package analysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-toolkit/watchdog/utils"
	"io"
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
	if baseURL == "" || apiKey == "" {
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
		return raw, err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return raw, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := utils.DoRequestWithRetry(req)
	if err != nil {
		return raw, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return raw, err
	}
	if resp.StatusCode != http.StatusOK {
		return raw, fmt.Errorf("llm http status %d: %s", resp.StatusCode, string(body))
	}

	var completion chatCompletionResponse
	if err = json.Unmarshal(body, &completion); err != nil {
		return raw, err
	}
	if len(completion.Choices) == 0 {
		return raw, nil
	}

	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	if content == "" {
		return raw, nil
	}
	return content, nil
}
