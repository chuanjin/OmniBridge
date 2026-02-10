package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/chuanjin/OmniBridge/internal/logger"
	"github.com/chuanjin/OmniBridge/internal/parser"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Define flags
	provider := flag.String("provider", "gemini", "LLM Provider (gemini, ollama)")
	model := flag.String("model", "", "Model Name (default: gemini-2.0-flash for gemini, deepseek-coder:1.3b for ollama)")
	endpoint := flag.String("endpoint", "", "API Endpoint")
	mode := flag.String("mode", "simulate", "Mode (simulate, server)")
	addr := flag.String("addr", ":8080", "TCP Server Address (only used in server mode)")
	debug := flag.Bool("debug", false, "Enable debug logging")

	flag.Parse()

	// Initialize Logger
	if err := logger.Init(*debug); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting OmniBridge Gateway...")

	// Load .env file
	err := godotenv.Load()
	if err != nil {
		logger.Warn("No .env file found, using system environment variables")
	}

	// 1. Initialize the Manager (Persistence) and Dispatcher (Routing)
	mgr := parser.NewParserManager("./storage", "./seeds")
	if err := mgr.SeedParsers(); err != nil {
		logger.Error("Failed to seed parsers", zap.Error(err))
	}

	// Load stored parsers and auto-bind those that have a // Signature: comment
	bindings, err := mgr.LoadSavedParsers()
	if err != nil {
		logger.Error("Error loading parsers", zap.Error(err))
	}

	dispatcher := parser.NewDispatcher(mgr)

	// Bind from code-extracted signatures
	for name, sigHex := range bindings {
		sig := hexToBytes(sigHex)
		dispatcher.Bind(sig, name)
		logger.Info("Auto-Bound parser", zap.String("signature", fmt.Sprintf("0x%X", sig)), zap.String("protocol", name))
	}

	// Also restore from manifest.json for any that don't have source signatures
	manifest, err := mgr.LoadManifest()
	if err == nil {
		for sigHex, name := range manifest {
			sig := hexToBytes(sigHex)
			dispatcher.Bind(sig, name) // Will overwrite if already bound, which is fine
		}
	}

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
			logger.Fatal("Server failed", zap.Error(err))
		}
		return
	}

	// 4. Simulated Data Stream (Original Loop)
	logger.Info("OmniBridge Gateway Started (SIMULATION MODE)")
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
			logger.Warn("Detected error in protocol", zap.String("protocol", proto), zap.Error(err))
			logger.Info("Attempting repair", zap.String("protocol", proto))

			// Get the faulty code from the manager to send back to the AI
			faultyCode, exists := mgr.GetParserCode(proto)
			if !exists {
				logger.Error("Could not find code for protocol to repair", zap.String("protocol", proto))
				continue
			}

			// With generalized discovery, we can just pass nil or empty signature
			// if we want the AI to re-verify it, or use the one we know.
			sig := []byte(nil)

			_, repairErr := discovery.RepairParser(proto, faultyCode, err.Error(), raw, sig)
			if repairErr != nil {
				logger.Error("Repair failed", zap.Error(repairErr))
				continue
			}

			// Re-attempt ingestion after repair
			result, proto, err = dispatcher.Ingest(raw)
			if err == nil {
				logger.Info("Protocol repaired successfully", zap.String("protocol", proto))
			}
		}

		// 6. DISCOVERY: If protocol is entirely unknown
		if err != nil && proto == "" {
			logger.Info("Unknown signature, consulting AI", zap.String("signature", fmt.Sprintf("0x%X", raw[0])))

			// Trigger Discovery Mode
			// Trigger Discovery Mode WITHOUT hardcoded signatures
			// The AI will now identify the signature from the raw data.
			context := "Industrial Voltage Sensor. Byte 0 is Signature, Byte 1-2 is Big-Endian Voltage (mV)."
			newName, discErr := discovery.DiscoverNewProtocol(raw, nil, context)

			if discErr != nil {
				logger.Error("Discovery failed", zap.Error(discErr))
				continue
			}

			// Re-attempt Ingestion
			result, proto, _ = dispatcher.Ingest(raw)
			logger.Info("New Protocol Learned", zap.String("protocol", newName))
		}

		if err == nil {
			logger.Info("Success", zap.String("protocol", proto), zap.Any("data", result))
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
