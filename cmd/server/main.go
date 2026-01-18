package main

import (
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
		for sig, name := range manifest {
			dispatcher.Bind(sig, name)
			fmt.Printf("📦 Restored Binding: 0x%02X -> %s\n", sig, name)
		}
	}

	// 2. Configure Local LLM (Ollama)
	// Using deepseek-coder:1.3b is the right choice for speed and code accuracy.

	// cfg := parser.DiscoveryConfig{
	// 	Provider: "ollama",
	// 	Endpoint: "http://localhost:11434/api/generate",
	// 	Model:    "deepseek-coder:1.3b",
	// }

	cfg := parser.DiscoveryConfig{
		Provider: "gemini",
		Model:    "gemini-2.0-flash",
	}
	discovery := parser.NewDiscoveryService(dispatcher, mgr, cfg)

	engineCode := `package dynamic
	func Parse(data []byte) map[string]interface{} {
		return map[string]interface{}{"rpm": int(data[1]) * 100}
	}`

	mgr.RegisterParser("Engine_System", engineCode)
	// 3. Pre-bind a known protocol for testing the fast-path
	dispatcher.Bind(0x01, "Engine_System")

	// 4. Simulated Data Stream
	// 0x01: Known Engine data
	// 0x2A: NEW! Unknown signature. Let's imagine this is a High-Precision Voltage sensor
	// where the next two bytes [0x03, 0xE8] represent 1000mV (Big-Endian).
	incomingStream := [][]byte{
		{0x01, 0x64},             // Known
		{0x2A, 0x03, 0xE8, 0xFF}, // UNKNOWN - Will trigger Discovery
	}

	fmt.Println("🚀 OmniBridge Gateway Started")
	fmt.Println("--------------------------------------------")

	for _, raw := range incomingStream {
		// Attempt to parse using cached/known logic
		result, proto, err := dispatcher.Ingest(raw)
		if err != nil {
			fmt.Printf("🔍 Error Ingesting [0x%X]: %v. Consulting Local AI...\n", raw[0], err)

			// 5. Trigger Discovery Mode
			// We provide a rich context hint to help the AI write better code
			context := "Industrial Voltage Sensor. Byte 0 is Signature, Byte 1-2 is Big-Endian Voltage (mV)."
			newName, discErr := discovery.DiscoverNewProtocol(raw, context)

			if discErr != nil {
				fmt.Printf("❌ Discovery failed: %v\n", discErr)
				continue
			}

			// 6. Re-attempt Ingestion
			// Now the manager has the new .go file and the dispatcher is bound.
			result, proto, _ = dispatcher.Ingest(raw)
			fmt.Printf("✨ New Protocol Learned & Persistent: %s\n", newName)
		}

		fmt.Printf("✅ [SUCCESS] Protocol: %-15s | Data: %v\n", proto, result)
	}

	fmt.Println("--------------------------------------------")
	fmt.Println("Done. Check the ./storage folder for the generated Go parsers.")
}
