package parser

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoveryService_DiscoverNewProtocol_Ollama(t *testing.T) {
	// 1. Setup mock Ollama server
	mockResponse := OllamaResponse{
		Response: `// Signature: 01AA
package dynamic

func Parse(data []byte) map[string]interface{} {
	return map[string]interface{}{
		"status": "ok",
		"value": int(data[0]),
	}
}`,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		var req OllamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	// Create agents directory for system_prompt.md if it doesn't exist
	if err := os.MkdirAll("agents", 0755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}
	defer func() { _ = os.RemoveAll("agents") }()

	if err := os.WriteFile("agents/system_prompt.md", []byte("System prompt context"), 0644); err != nil {
		t.Fatalf("Failed to write system_prompt.md: %v", err)
	}

	// 2. Setup DiscoveryService
	// Create a temp storage and seed paths
	tempDir, err := os.MkdirTemp("", "omnibridge_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	storagePath := filepath.Join(tempDir, "storage")
	seedPath := filepath.Join(tempDir, "seed")
	_ = os.MkdirAll(storagePath, 0755)
	_ = os.MkdirAll(seedPath, 0755)

	manager := NewParserManager(storagePath, seedPath)
	dispatcher := NewDispatcher(manager)

	cfg := DiscoveryConfig{
		Provider: "ollama",
		Endpoint: server.URL,
		Model:    "llama3",
	}
	service := NewDiscoveryService(dispatcher, manager, cfg)

	// 3. Test Discovery
	rawSample := []byte{0x01, 0xAA, 0x02, 0x03}
	signature := []byte{0x01, 0xAA}

	protocolID, err := service.DiscoverNewProtocol(rawSample, signature, "test hint")
	if err != nil {
		t.Fatalf("DiscoverNewProtocol failed: %v", err)
	}

	expectedID := "auto_proto_0x01AA"
	if protocolID != expectedID {
		t.Errorf("Expected protocol ID %s, got %s", expectedID, protocolID)
	}

	// 4. Verify dispatcher binding by ingesting data
	result, matchedProto, err := dispatcher.Ingest(rawSample)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if matchedProto != expectedID {
		t.Errorf("Expected matched protocol %s, got %s", expectedID, matchedProto)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status ok, got %v", result["status"])
	}

	// data[0] is 0x01 (1)
	if fmt.Sprintf("%v", result["value"]) != "1" {
		t.Errorf("Expected value 1, got %v", result["value"])
	}
}

func TestDiscoveryService_DiscoverNewProtocol_Gemini(t *testing.T) {
	// 1. Setup mock Gemini server
	mockResponse := struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}{
		Candidates: []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		}{
			{
				Content: struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				}{
					Parts: []struct {
						Text string `json:"text"`
					}{
						{Text: `// Signature: 02BB
package dynamic
func Parse(data []byte) map[string]interface{} {
	return map[string]interface{}{"status": "gemini_mock", "value": int(data[0])}
}`},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	_ = os.Setenv("GEMINI_API_KEY", "test-key")
	defer func() { _ = os.Unsetenv("GEMINI_API_KEY") }()

	// Setup agents
	if err := os.MkdirAll("agents", 0755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}
	defer func() { _ = os.RemoveAll("agents") }()

	if err := os.WriteFile("agents/system_prompt.md", []byte("System prompt context"), 0644); err != nil {
		t.Fatalf("Failed to write system_prompt.md: %v", err)
	}

	// 2. Setup DiscoveryService
	tempDir, _ := os.MkdirTemp("", "omnibridge_gemini_test")
	defer func() { _ = os.RemoveAll(tempDir) }()

	manager := NewParserManager(filepath.Join(tempDir, "storage"), filepath.Join(tempDir, "seed"))
	dispatcher := NewDispatcher(manager)

	cfg := DiscoveryConfig{
		Provider: "gemini",
		Endpoint: server.URL,
		Model:    "gemini-pro",
	}
	service := NewDiscoveryService(dispatcher, manager, cfg)

	// 3. Test Discovery
	rawSample := []byte{0x02, 0xBB, 0x01}
	signature := []byte{0x02, 0xBB}

	protocolID, err := service.DiscoverNewProtocol(rawSample, signature, "test hint")
	if err != nil {
		t.Fatalf("DiscoverNewProtocol failed: %v", err)
	}

	expectedID := "auto_proto_0x02BB"
	if protocolID != expectedID {
		t.Errorf("Expected protocol ID %s, got %s", expectedID, protocolID)
	}

	// 4. Verify binding
	result, _, err := dispatcher.Ingest(rawSample)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if result["status"] != "gemini_mock" {
		t.Errorf("Expected status gemini_mock, got %v", result["status"])
	}
}

func TestDiscoveryService_RetryLogic(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, "transient error")
			return
		}

		mockResponse := OllamaResponse{
			Response: `// Signature: 03CC
package dynamic
func Parse(data []byte) map[string]interface{} {
	return map[string]interface{}{"status": "retry_ok"}
}`,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	// Setup agents
	if err := os.MkdirAll("agents", 0755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}
	defer func() { _ = os.RemoveAll("agents") }()

	if err := os.WriteFile("agents/system_prompt.md", []byte("System prompt context"), 0644); err != nil {
		t.Fatalf("Failed to write system_prompt.md: %v", err)
	}

	tempDir, _ := os.MkdirTemp("", "omnibridge_retry_test")
	defer func() { _ = os.RemoveAll(tempDir) }()

	manager := NewParserManager(filepath.Join(tempDir, "storage"), filepath.Join(tempDir, "seed"))
	dispatcher := NewDispatcher(manager)

	cfg := DiscoveryConfig{
		Provider:   "ollama",
		Endpoint:   server.URL,
		Model:      "llama3",
		MaxRetries: 3,
		RetryDelay: 10 * time.Millisecond, // Short delay for testing
	}
	service := NewDiscoveryService(dispatcher, manager, cfg)

	rawSample := []byte{0x03, 0xCC, 0x01}
	signature := []byte{0x03, 0xCC}

	protocolID, err := service.DiscoverNewProtocol(rawSample, signature, "test retry")
	if err != nil {
		t.Fatalf("DiscoverNewProtocol failed after retries: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	if protocolID != "auto_proto_0x03CC" {
		t.Errorf("Expected protocol ID auto_proto_0x03CC, got %s", protocolID)
	}
}
