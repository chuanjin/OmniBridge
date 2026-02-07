package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParserManager_RegisterAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewParserManager(tmpDir, "")

	// 1. Register a new parser
	protoID := "test_proto"
	validCode := `package dynamic
import "fmt"
// Signature: AABB
func Parse(data []byte) map[string]interface{} {
	return map[string]interface{}{"status": "ok"}
}
`
	err = mgr.RegisterParser(protoID, validCode)
	if err != nil {
		t.Fatalf("RegisterParser failed: %v", err)
	}

	// 2. Verify file exists
	expectedPath := filepath.Join(tmpDir, protoID+".go")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Parser file was not created at %s", expectedPath)
	}

	// 3. Create a new manager instance to simulate restart and test specific LoadSavedParsers functionality
	mgr2 := NewParserManager(tmpDir, "")
	loaded, err := mgr2.LoadSavedParsers()
	if err != nil {
		t.Fatalf("LoadSavedParsers failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Errorf("Expected 1 loaded parser, got %d", len(loaded))
	}
	if loaded[protoID] != "AABB" {
		t.Errorf("Expected signature AABB, got %s", loaded[protoID])
	}

	// Check if code is in cache
	cachedCode, exists := mgr2.GetParserCode(protoID)
	if !exists {
		t.Error("Code not found in cache after loading")
	}
	if cachedCode != validCode {
		t.Error("Cached code does not match registered code")
	}
}

func TestParserManager_Manifest(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "manifest_test")
	defer os.RemoveAll(tmpDir)

	mgr := NewParserManager(tmpDir, "")

	bindings := map[string]string{
		"01": "Proto1",
		"02": "Proto2",
	}

	// Save
	err := mgr.SaveManifest(bindings)
	if err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	// Load
	loadedBindings, err := mgr.LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if len(loadedBindings) != 2 {
		t.Errorf("Expected 2 bindings, got %d", len(loadedBindings))
	}
	if loadedBindings["01"] != "Proto1" {
		t.Errorf("Binding mismatch")
	}
}

func TestParserManager_Manifest_Empty(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "manifest_empty_test")
	defer os.RemoveAll(tmpDir)

	mgr := NewParserManager(tmpDir, "")

	// Load without saving first
	loadedBindings, err := mgr.LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest on empty dir failed: %v", err)
	}

	if len(loadedBindings) != 0 {
		t.Errorf("Expected empty bindings, got %d", len(loadedBindings))
	}
}
