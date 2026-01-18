package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func CallOpenAI(prompt string) (string, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		// fallback: return truncated prompt as placeholder answer
		if len(prompt) > 1500 {
			return prompt[:1500], nil
		}
		return prompt, nil
	}

	payload := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个基于给定文档回答问题的助手。请使用简洁、中文的回答。"},
			{"role": "user", "content": prompt},
		},
		"max_tokens":  500,
		"temperature": 0.2,
	}

	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var r map[string]interface{}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}

	if choices, ok := r["choices"].([]interface{}); ok && len(choices) > 0 {
		if ch, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := ch["message"].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					return content, nil
				}
			}
		}
	}

	return fmt.Sprintf("unexpected openai response: %s", string(body)), nil
}
