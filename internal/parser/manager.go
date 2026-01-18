package parser

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

type ParserManager struct {
	engine      *Engine
	storagePath string
	cache       map[string]string // ProtocolID -> GoCode
	mu          sync.RWMutex
}

func NewParserManager(storagePath string) *ParserManager {
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		os.MkdirAll(storagePath, 0o755)
	}
	return &ParserManager{
		engine:      NewEngine(),
		storagePath: storagePath,
		cache:       make(map[string]string),
	}
}

// LoadSavedParsers reads all .go files from the storage folder on startup
func (m *ParserManager) LoadSavedParsers() error {
	files, err := ioutil.ReadDir(m.storagePath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".go" {
			protocolID := file.Name()[:len(file.Name())-3]
			content, _ := ioutil.ReadFile(filepath.Join(m.storagePath, file.Name()))
			m.cache[protocolID] = string(content)
			fmt.Printf("📦 Loaded cached parser for: %s\n", protocolID)
		}
	}
	return nil
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
	Bindings map[uint8]string `json:"bindings"`
}

// SaveManifest writes the current dispatcher bindings to a JSON file
func (m *ParserManager) SaveManifest(bindings map[uint8]string) error {
	manifest := Manifest{Bindings: bindings}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(m.storagePath, "manifest.json")
	return ioutil.WriteFile(path, data, 0o644)
}

// LoadManifest reads the manifest.json and returns the bindings
func (m *ParserManager) LoadManifest() (map[uint8]string, error) {
	path := filepath.Join(m.storagePath, "manifest.json")

	// If file doesn't exist, return empty map (common on first run)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return make(map[uint8]string), nil
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
