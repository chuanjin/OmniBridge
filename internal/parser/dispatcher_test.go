package parser

import (
	"os"
	"testing"
)

func TestDispatcher_BindAndIngest(t *testing.T) {
	// Setup a temporary manager (we don't need real storage for this test, but Manager expects paths)
	tmpDir, _ := os.MkdirTemp("", "omnibridge_test")
	defer os.RemoveAll(tmpDir)

	mgr := NewParserManager(tmpDir, "")
	d := NewDispatcher(mgr)

	// Define some signatures
	sig1 := []byte{0x01}
	sig2 := []byte{0x01, 0x02} // Longer prefix

	// Bind them
	d.Bind(sig1, "Proto1")
	d.Bind(sig2, "Proto2")

	// Test case 1: Exact match for Sig1 (should match Proto1 if data is short, 
    // but wait, prefix logic says: 0x01 matches Proto1. 
    // If input is 0x01 0x03, it matches Proto1.
	// If input is 0x01 0x02, it matches Proto2 (longest prefix).

	tests := []struct {
		name          string
		input         []byte
		expectedProto string
	}{
		{
			name:          "Match Proto1",
			input:         []byte{0x01, 0x03, 0xFF},
			expectedProto: "Proto1",
		},
		{
			name:          "Match Proto2 (Longest Prefix)",
			input:         []byte{0x01, 0x02, 0xFF},
			expectedProto: "Proto2",
		},
		{
			name:          "Unknown Protocol",
			input:         []byte{0xFF, 0xAA},
			expectedProto: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// specific parser won't exist, so Ingest returns error from ParseData
			// but we only care about the matchedProto return value for this test
			_, matchedProto, _ := d.Ingest(tt.input)
			
			if matchedProto != tt.expectedProto {
				t.Errorf("Ingest() matchedProto = %v, want %v", matchedProto, tt.expectedProto)
			}
		})
	}
}

func TestDispatcher_GetBindings(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "omnibridge_test")
	defer os.RemoveAll(tmpDir)
	mgr := NewParserManager(tmpDir, "")
	d := NewDispatcher(mgr)

	d.Bind([]byte{0xAA}, "ProtoA")
	
	bindings := d.GetBindings()
	if len(bindings) != 1 {
		t.Errorf("Expected 1 binding, got %d", len(bindings))
	}
	if bindings["AA"] != "ProtoA" {
		t.Errorf("Expected binding for AA to be ProtoA, got %v", bindings["AA"])
	}
}
