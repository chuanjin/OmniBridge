package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"

	"github.com/chuanjin/OmniBridge/internal/parser"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Note: No .env file found, using system environment variables")
	}

	// 1. Initialize the Manager (Persistence) and Dispatcher (Routing)
	mgr := parser.NewParserManager("./storage")
	if err := mgr.LoadSavedParsers(); err != nil {
		log.Printf("Note: No existing parsers found in storage: %v", err)
	}

	dispatcher := parser.NewDispatcher(mgr)

	// Automatically restore persistent bindings from manifest.json
	manifest, err := mgr.LoadManifest()
	if err == nil {
		for sigHex, name := range manifest {
			sig := hexToBytes(sigHex)
			dispatcher.Bind(sig, name)
			fmt.Printf("📦 Restored Binding: 0x%X -> %s\n", sig, name)
		}
	}

	// 2. Configure Local/Cloud LLM via CLI flags
	provider := flag.String("provider", "gemini", "LLM Provider (gemini, ollama)")
	model := flag.String("model", "", "Model Name (default: gemini-2.0-flash for gemini, deepseek-coder:1.3b for ollama)")
	endpoint := flag.String("endpoint", "", "API Endpoint")
	flag.Parse()

	// Set defaults based on provider if not specified
	effectiveModel := *model
	if effectiveModel == "" {
		if *provider == "ollama" {
			effectiveModel = "deepseek-coder:1.3b"
		} else {
			effectiveModel = "gemini-2.0-flash"
		}
	}

	effectiveEndpoint := *endpoint
	if effectiveEndpoint == "" {
		if *provider == "ollama" {
			effectiveEndpoint = "http://localhost:11434/api/generate"
		} else {
			effectiveEndpoint = "https://generativelanguage.googleapis.com/v1beta/models"
		}
	}

	cfg := parser.DiscoveryConfig{
		Provider: *provider,
		Model:    effectiveModel,
		Endpoint: effectiveEndpoint,
	}
	discovery := parser.NewDiscoveryService(dispatcher, mgr, cfg)

	engineCode := `package dynamic
	func Parse(data []byte) map[string]interface{} {
		return map[string]interface{}{"rpm": int(data[1]) * 100}
	}`

	mgr.RegisterParser("Engine_System", engineCode)
	// 3. Pre-bind a known protocol for testing the fast-path
	dispatcher.Bind([]byte{0x01}, "Engine_System")

	// 4. Simulated Data Stream
	// 0x01: Known Engine data
	// 0x55, 0xAA: NEW! Multi-byte signature.
	// 0x2A: Known from previous tests (will trigger repair/discovery if not in storage)
	incomingStream := [][]byte{
		{0x01, 0x64},                   // Single-byte match
		{0x55, 0xAA, 0x03, 0xE8, 0xFF}, // MULTI-BYTE Signature - Will trigger Discovery
		{0x2A, 0x01, 0xF4},             // Known or Discovery (added 0xF4 to make it 3 bytes)
	}

	fmt.Println("🚀 OmniBridge Gateway Started")
	fmt.Println("--------------------------------------------")

	for _, raw := range incomingStream {
		// Attempt to parse using cached/known logic
		result, proto, err := dispatcher.Ingest(raw)

		// 5. SELF-HEALING: If ingest fails for a KNOWN protocol (e.g., compile error), try to repair it
		if err != nil && proto != "" {
			fmt.Printf("🔧 Detected error in [0x%X] (%s): %v. Attempting repair...\n", raw[0], proto, err)

			// Get the faulty code from the manager to send back to the AI
			faultyCode, exists := mgr.GetParserCode(proto)
			if !exists {
				fmt.Printf("❌ Could not find code for protocol %s to repair\n", proto)
				continue
			}

			// In a real app, we'd extract the sig that matched this proto.
			// For this demo, we'll just use the first byte or a hardcoded one for known cases.
			sig := []byte{raw[0]}
			if proto == "auto_proto_0x55AA" {
				sig = []byte{0x55, 0xAA}
			}

			_, repairErr := discovery.RepairParser(proto, faultyCode, err.Error(), raw, sig)
			if repairErr != nil {
				fmt.Printf("❌ Repair failed: %v\n", repairErr)
				continue
			}

			// Re-attempt ingestion after repair
			result, proto, err = dispatcher.Ingest(raw)
			if err == nil {
				fmt.Printf("✨ Protocol %s repaired successfully!\n", proto)
			}
		}

		// 6. DISCOVERY: If protocol is entirely unknown
		if err != nil && proto == "" {
			fmt.Printf("🔍 Error Ingesting [0x%X]: %v. Consulting Local AI...\n", raw[0], err)

			// Trigger Discovery Mode
			// For the 0x55 0xAA sample, we want to specify it's a 2-byte signature
			sig := []byte{raw[0]}
			if len(raw) > 1 && raw[0] == 0x55 && raw[1] == 0xAA {
				sig = []byte{0x55, 0xAA}
			}

			context := "Industrial Voltage Sensor. Byte 0 is Signature, Byte 1-2 is Big-Endian Voltage (mV)."
			newName, discErr := discovery.DiscoverNewProtocol(raw, sig, context)

			if discErr != nil {
				fmt.Printf("❌ Discovery failed: %v\n", discErr)
				continue
			}

			// Re-attempt Ingestion
			result, proto, _ = dispatcher.Ingest(raw)
			fmt.Printf("✨ New Protocol Learned & Persistent: %s\n", newName)
		}

		if err == nil {
			fmt.Printf("✅ [SUCCESS] Protocol: %-15s | Data: %v\n", proto, result)
		}
	}

	fmt.Println("--------------------------------------------")
	fmt.Println("Done. Check the ./storage folder for the generated Go parsers.")
}

func hexToBytes(h string) []byte {
	if len(h)%2 != 0 {
		h = "0" + h
	}
	b, _ := hex.DecodeString(h)
	return b
}
