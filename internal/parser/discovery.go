package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// DiscoveryService handles the interaction with LLMs to generate new parsers
type DiscoveryService struct {
	dispatcher *Dispatcher
	manager    *ParserManager
	httpClient *http.Client
	Config     DiscoveryConfig
}

type DiscoveryConfig struct {
	Provider    string // "ollama" or "anthropic"
	Endpoint    string // e.g., "http://localhost:11434/api/generate"
	Model       string // e.g., "llama3" or "deepseek-coder"
	ApiKey      string // Optional for local, required for cloud
	PrivacyMode bool   // If true, masks potential PII before sending
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Response string `json:"response"`
}

func NewDiscoveryService(d *Dispatcher, m *ParserManager, cfg DiscoveryConfig) *DiscoveryService {
	return &DiscoveryService{
		dispatcher: d,
		manager:    m,
		httpClient: &http.Client{Timeout: 600 * time.Second},
		Config:     cfg,
	}
}

func (s *DiscoveryService) DiscoverNewProtocol(rawSample []byte, signature []byte, contextHint string) (string, error) {
	if len(signature) == 0 {
		signature = []byte{rawSample[0]}
	}
	fmt.Printf("🔒 Discovery Mode [%s]: Analyzing signature 0x%X...\n", s.Config.Provider, signature)

	// 1. Load System Prompt from the agents folder
	absPath, _ := filepath.Abs("agents/system_prompt.md")
	systemPrompt, err := ioutil.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to load system_prompt.md: %v", err)
	}

	// 2. Combine with the specific instance data
	fullPrompt := fmt.Sprintf("%s\n\nINPUT:\nHex Sample: %X\nProtocol Hints: %s",
		string(systemPrompt), rawSample, contextHint)

	return s.requestAndRegister(fullPrompt, signature)
}

func (s *DiscoveryService) RepairParser(protocolID string, faultyCode string, errorMsg string, rawSample []byte, signature []byte) (string, error) {
	fmt.Printf("🔧 Repair Mode [%s]: Fixing protocol %s...\n", s.Config.Provider, protocolID)

	absPath, _ := filepath.Abs("agents/system_prompt.md")
	systemPrompt, err := ioutil.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to load system_prompt.md: %v", err)
	}

	fullPrompt := fmt.Sprintf("%s\n\n### ERROR TO FIX\nYou previously generated code that failed.\n\nFAULTY CODE:\n```go\n%s\n```\n\nERROR MESSAGE:\n%s\n\nINPUT DATA (Hex): %X\n\nPlease fix the code and return only the valid Go code.",
		string(systemPrompt), faultyCode, errorMsg, rawSample)

	if len(signature) == 0 {
		signature = []byte{rawSample[0]}
	}

	return s.requestAndRegister(fullPrompt, signature)
}

func (s *DiscoveryService) requestAndRegister(prompt string, signature []byte) (string, error) {
	var generatedCode string
	var err error

	// 3. Route to provider (Ollama/Cloud)
	if s.Config.Provider == "ollama" {
		generatedCode, err = s.callOllama(prompt)
	} else {
		generatedCode, err = s.callCloud(prompt)
	}

	if err != nil {
		return "", err
	}

	// 4. Register and Bind
	protocolID := fmt.Sprintf("auto_proto_0x%X", signature)

	cleanCode := sanitizeAiCode(generatedCode)
	// Register the CLEAN code
	err = s.manager.RegisterParser(protocolID, cleanCode)
	if err != nil {
		return "", err
	}

	s.dispatcher.Bind(signature, protocolID)

	// Persist the new binding to the manifest file
	s.manager.SaveManifest(s.dispatcher.GetBindings())
	return protocolID, nil
}

func (s *DiscoveryService) callOllama(prompt string) (string, error) {
	reqBody := OllamaRequest{
		Model:  s.Config.Model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, _ := json.Marshal(reqBody)
	fmt.Println("⏳ LLM is thinking (this may take a minute)...")
	resp, err := s.httpClient.Post(s.Config.Endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var ollamaResp OllamaResponse
	json.Unmarshal(body, &ollamaResp)

	return ollamaResp.Response, nil
}

func (s *DiscoveryService) callCloud(prompt string) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}

	// Construct URL dynamically using Endpoint and Model
	// Default Endpoint: https://generativelanguage.googleapis.com/v1beta/models
	// Format: <Endpoint>/<Model>:generateContent?key=<ApiKey>
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", s.Config.Endpoint, s.Config.Model, apiKey)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.1, // Low temperature for code precision
			"maxOutputTokens": 1024,
		},
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("gemini api error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text, nil
	}

	return "", fmt.Errorf("no content returned from gemini")
}

func sanitizeAiCode(input string) string {
	// 1. Force remove any "Here is your code" or preamble
	// We look for the first occurrence of "package dynamic"
	start := strings.Index(input, "package dynamic")
	if start != -1 {
		input = input[start:]
	}

	// 2. Clean up common AI hallucinations in the function name
	// Some models try to name it ParseID99 or ParseHex...
	// We force it back to 'Parse' using regex
	reFuncName := regexp.MustCompile(`func [A-Za-z0-9_]+\(`)
	input = reFuncName.ReplaceAllString(input, "func Parse(")

	// 3. The "Brace Matcher": Only keep from 'package dynamic' to the LAST '}'
	lastBrace := strings.LastIndex(input, "}")
	if lastBrace != -1 {
		input = input[:lastBrace+1]
	}

	// 4. Final safety check: if the AI skipped 'package dynamic', prepend it
	if !strings.Contains(input, "package dynamic") {
		input = "package dynamic\n\n" + input
	}

	return strings.TrimSpace(input)
}
