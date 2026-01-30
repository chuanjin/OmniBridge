package parser

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

type ParserManager struct {
	engine      *Engine
	storagePath string
	seedPath    string
	cache       map[string]string // ProtocolID -> GoCode
	mu          sync.RWMutex
}

func NewParserManager(storagePath string, seedPath string) *ParserManager {
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		os.MkdirAll(storagePath, 0o755)
	}
	return &ParserManager{
		engine:      NewEngine(),
		storagePath: storagePath,
		seedPath:    seedPath,
		cache:       make(map[string]string),
	}
}

// SeedParsers copies files from seedPath to storagePath if they don't exist
func (m *ParserManager) SeedParsers() error {
	if m.seedPath == "" {
		return nil
	}

	files, err := ioutil.ReadDir(m.seedPath)
	if err != nil {
		return nil // Ignore if seed path doesn't exist
	}

	for _, file := range files {
		destPath := filepath.Join(m.storagePath, file.Name())
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			content, err := ioutil.ReadFile(filepath.Join(m.seedPath, file.Name()))
			if err == nil {
				ioutil.WriteFile(destPath, content, 0o644)
				fmt.Printf("🌱 Seeded parser: %s\n", file.Name())
			}
		}
	}
	return nil
}

// LoadSavedParsers reads all .go files from the storage folder on startup
// Returns a map of ProtocolID -> SignatureHex
func (m *ParserManager) LoadSavedParsers() (map[string]string, error) {
	files, err := ioutil.ReadDir(m.storagePath)
	if err != nil {
		return nil, err
	}

	bindings := make(map[string]string)
	reSig := regexp.MustCompile(`// Signature:\s*([0-9A-Fa-f]+)`)

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".go" {
			protocolID := file.Name()[:len(file.Name())-3]
			content, _ := ioutil.ReadFile(filepath.Join(m.storagePath, file.Name()))
			code := string(content)
			m.cache[protocolID] = code

			// Extract signature from code comments
			matches := reSig.FindStringSubmatch(code)
			if len(matches) > 1 {
				bindings[protocolID] = matches[1]
			}

			fmt.Printf("📦 Loaded cached parser for: %s\n", protocolID)
		}
	}
	return bindings, nil
}

// RegisterParser saves a new AI-generated parser to disk and cache
func (m *ParserManager) RegisterParser(protocolID, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filename := filepath.Join(m.storagePath, protocolID+".go")
	err := ioutil.WriteFile(filename, []byte(code), 0o644)
	if err != nil {
		return err
	}

	m.cache[protocolID] = code
	return nil
}

// GetParserCode returns the source code for a given protocol ID
func (m *ParserManager) GetParserCode(protocolID string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	code, exists := m.cache[protocolID]
	return code, exists
}

// ParseData executes the parser at native speed from cache
func (m *ParserManager) ParseData(protocolID string, data []byte) (map[string]interface{}, error) {
	m.mu.RLock()
	code, exists := m.cache[protocolID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no parser found for %s. Please trigger AI generation", protocolID)
	}

	// Native speed execution via Interpreter
	return m.engine.Execute(data, code)
}

// Manifest represents the persistent mapping of signatures to parser IDs
type Manifest struct {
	Bindings map[string]string `json:"bindings"`
}

// SaveManifest writes the current dispatcher bindings to a JSON file
func (m *ParserManager) SaveManifest(bindings map[string]string) error {
	manifest := Manifest{Bindings: bindings}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(m.storagePath, "manifest.json")
	return ioutil.WriteFile(path, data, 0o644)
}

// LoadManifest reads the manifest.json and returns the bindings
func (m *ParserManager) LoadManifest() (map[string]string, error) {
	path := filepath.Join(m.storagePath, "manifest.json")

	// If file doesn't exist, return empty map (common on first run)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return make(map[string]string), nil
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return manifest.Bindings, nil
}
