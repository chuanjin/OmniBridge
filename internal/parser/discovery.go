package parser

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chuanjin/OmniBridge/internal/logger"
	"go.uber.org/zap"
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
	MaxRetries  int    // Maximum number of retries for LLM calls
	RetryDelay  time.Duration
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
	logger.Info("Discovery Mode: Analyzing signature", zap.String("provider", s.Config.Provider), zap.String("signature", fmt.Sprintf("0x%X", signature)))

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
	logger.Info("Repair Mode: Fixing protocol", zap.String("provider", s.Config.Provider), zap.String("protocol", protocolID))

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

	maxRetries := s.Config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1 // Default to at least one attempt
	}
	retryDelay := s.Config.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 2 * time.Second // Default initial delay
	}

	for i := 0; i < maxRetries; i++ {
		// 3. Route to provider (Ollama/Cloud)
		if s.Config.Provider == "ollama" {
			generatedCode, err = s.callOllama(prompt)
		} else {
			generatedCode, err = s.callCloud(prompt)
		}

		if err == nil {
			break
		}

		if i < maxRetries-1 {
			logger.Warn("LLM request failed, retrying", zap.Int("attempt", i+1), zap.Int("max_retries", maxRetries), zap.Error(err), zap.Duration("retry_delay", retryDelay))
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		} else {
			return "", fmt.Errorf("all LLM attempts failed: %v", err)
		}
	}

	// 4. Extract Signature from code if it exists (// Signature: 01AA)
	reSig := regexp.MustCompile(`// Signature:\s*([0-9A-Fa-f]+)`)
	matches := reSig.FindStringSubmatch(generatedCode)

	finalSig := signature
	if len(matches) > 1 {
		hexStr := matches[1]
		if len(hexStr)%2 != 0 {
			hexStr = "0" + hexStr
		}
		sigBytes, _ := hex.DecodeString(hexStr)
		if len(sigBytes) > 0 {
			finalSig = sigBytes
		}
	}

	if len(finalSig) == 0 {
		return "", fmt.Errorf("no signature found in AI response and none provided")
	}

	protocolID := fmt.Sprintf("auto_proto_0x%X", finalSig)

	cleanCode := sanitizeAiCode(generatedCode)
	// Register the CLEAN code
	err = s.manager.RegisterParser(protocolID, cleanCode)
	if err != nil {
		return "", err
	}

	s.dispatcher.Bind(finalSig, protocolID)

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
	logger.Debug("LLM is thinking...")
	resp, err := s.httpClient.Post(s.Config.Endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("ollama connection failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(body))
	}

	body, _ := ioutil.ReadAll(resp.Body)
	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode ollama response: %v", err)
	}

	if ollamaResp.Response == "" {
		return "", fmt.Errorf("ollama returned empty response")
	}

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
	resp, err := s.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("gemini connection failed: %v", err)
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
