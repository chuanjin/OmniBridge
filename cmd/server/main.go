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
	mgr := parser.NewParserManager("./storage", "./seeds")
	mgr.SeedParsers()

	// Load stored parsers and auto-bind those that have a // Signature: comment
	bindings, err := mgr.LoadSavedParsers()
	if err != nil {
		log.Printf("Note: Error loading parsers: %v", err)
	}

	dispatcher := parser.NewDispatcher(mgr)

	// Bind from code-extracted signatures
	for name, sigHex := range bindings {
		sig := hexToBytes(sigHex)
		dispatcher.Bind(sig, name)
		fmt.Printf("📦 Auto-Bound (from source): 0x%X -> %s\n", sig, name)
	}

	// Also restore from manifest.json for any that don't have source signatures
	manifest, err := mgr.LoadManifest()
	if err == nil {
		for sigHex, name := range manifest {
			sig := hexToBytes(sigHex)
			dispatcher.Bind(sig, name) // Will overwrite if already bound, which is fine
		}
	}

	// 2. Configure Local/Cloud LLM via CLI flags
	provider := flag.String("provider", "gemini", "LLM Provider (gemini, ollama)")
	model := flag.String("model", "", "Model Name (default: gemini-2.0-flash for gemini, deepseek-coder:1.3b for ollama)")
	endpoint := flag.String("endpoint", "", "API Endpoint")
	mode := flag.String("mode", "simulate", "Mode (simulate, server)")
	addr := flag.String("addr", ":8080", "TCP Server Address (only used in server mode)")
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

	// 3. Mode selection
	if *mode == "server" {
		srv := parser.NewTCPServer(*addr, dispatcher, discovery)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("❌ Server failed: %v", err)
		}
		return
	}

	// 4. Simulated Data Stream (Original Loop)
	fmt.Println("🚀 OmniBridge Gateway Started (SIMULATION MODE)")
	fmt.Println("--------------------------------------------")

	incomingStream := [][]byte{
		{0x01, 0x64},                   // Single-byte match (Legacy Engine_System)
		{0x41, 0x0C, 0x1A, 0xF8},       // Engine RPM (1726 RPM)
		{0x41, 0x04, 0x7F},             // Engine Load (49%)
		{0x41, 0x05, 0x5A},             // Coolant Temp (50°C)
		{0x41, 0x0D, 0x4B},             // Vehicle Speed (75 km/h)
		{0x41, 0x10, 0x0D, 0xAC},       // MAF Air Flow (35.00 g/s)
		{0x41, 0x11, 0xCC},             // Throttle Position (80%)
		{0x41, 0x0B, 0x64},             // Intake Pressure (100 kPa)
		{0x41, 0x0F, 0x3C},             // Intake Temp (20°C)
		{0x41, 0x21, 0x04, 0xD2},       // MIL Distance (1234 km)
		{0x41, 0x2F, 0x7F},             // Fuel Level (49%)
		{0x41, 0x33, 0x65},             // Baro Pressure (101 kPa)
		{0x55, 0xAA, 0x03, 0xE8, 0xFF}, // MULTI-BYTE Signature - Will trigger Discovery
		{0x2A, 0x01, 0xF4},             // Known or Discovery
		{0x99, 0xFF, 0x00, 0x01},       // NEW Signature
	}

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

			// With generalized discovery, we can just pass nil or empty signature
			// if we want the AI to re-verify it, or use the one we know.
			sig := []byte(nil)

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
			// Trigger Discovery Mode WITHOUT hardcoded signatures
			// The AI will now identify the signature from the raw data.
			context := "Industrial Voltage Sensor. Byte 0 is Signature, Byte 1-2 is Big-Endian Voltage (mV)."
			newName, discErr := discovery.DiscoverNewProtocol(raw, nil, context)

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
