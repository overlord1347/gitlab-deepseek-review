package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type ReviewRequest struct {
	Diff string `json:"diff"`
}

type ReviewResponse struct {
	Review string `json:"review"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIRequestBody struct {
	Model            string    `json:"model"`
	Stream           bool      `json:"stream"`
	MaxTokens        int       `json:"max_tokens"`
	EnableThinking   bool      `json:"enable_thinking"`
	ThinkingBudget   int       `json:"thinking_budget"`
	MinP             float64   `json:"min_p"`
	Temperature      float64   `json:"temperature"`
	TopP             float64   `json:"top_p"`
	TopK             int       `json:"top_k"`
	FrequencyPenalty float64   `json:"frequency_penalty"`
	N                int       `json:"n"`
	Stop             []string  `json:"stop"`
	Messages         []Message `json:"messages"`
}

type AIResponseChoice struct {
	Message Message `json:"message"`
}

type AIResponseBody struct {
	Choices []AIResponseChoice `json:"choices"`
}

func reviewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	var req ReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный JSON", http.StatusBadRequest)
		return
	}

	aiReq := AIRequestBody{
		Model:            "Qwen/Qwen2.5-VL-72B-Instruct",
		Stream:           false,
		MaxTokens:        4096,
		EnableThinking:   true,
		ThinkingBudget:   4096,
		MinP:             0.05,
		Temperature:      0.7,
		TopP:             0.7,
		TopK:             50,
		FrequencyPenalty: 0.5,
		N:                1,
		Stop:             []string{},
		Messages: []Message{
			{
				Role: "user",
				Content: fmt.Sprintf(`Ты — профессиональный ревьюер кода.
Проанализируй следующий diff:

%s

Выдай подробные замечания и рекомендации. Начни с комплемента разработчику за его работу, а потом выдай всю правду о его мерж реквесте, без стеснения, докапывайся даже до орфографических ошибок, но помни что приоритет номер 1 - ошибки в коде.  Ответ на русском, с красивой Markdown-разметкой для публикации в GitLab Merge Request: с заголовками, списками, примерами кода.`, req.Diff),
			},
		},
	}

	jsonData, err := json.Marshal(aiReq)
	if err != nil {
		http.Error(w, "Ошибка сериализации запроса", http.StatusInternalServerError)
		return
	}

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		http.Error(w, "API ключ не задан в переменной окружения SILICON_API_KEY", http.StatusInternalServerError)
		return
	}

	httpReq, err := http.NewRequest("POST", "https://api.siliconflow.cn/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, "Ошибка формирования запроса", http.StatusInternalServerError)
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 3000 * time.Second,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		http.Error(w, "Ошибка обращения к AI API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("AI API ответил с ошибкой: %s\n%s", resp.Status, body), http.StatusInternalServerError)
		return
	}

	var aiResp AIResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		http.Error(w, "Ошибка декодирования ответа AI", http.StatusInternalServerError)
		return
	}

	if len(aiResp.Choices) == 0 {
		http.Error(w, "Пустой ответ от AI", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(ReviewResponse{
		Review: aiResp.Choices[0].Message.Content,
	})
}

func main() {
	http.HandleFunc("/review", reviewHandler)
	http.ListenAndServe(":7076", nil)
}
