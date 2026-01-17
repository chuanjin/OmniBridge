package main

import (
	"fmt"

	"github.com/chuanjin/OmniBridge/internal/parser"
)

func main() {
	// 1. Setup Engine & Storage
	mgr := parser.NewParserManager("./storage")
	mgr.LoadSavedParsers()
	dispatcher := parser.NewDispatcher(mgr)

	// 2. Register Protocols (This happens once during AI Learning)
	// Example: Signature 0x01 is for Engine, 0x02 is for Battery
	dispatcher.Bind(0x01, "Engine_System")
	dispatcher.Bind(0x02, "Battery_Pack")

	// 3. Simulate a stream of different protocols hitting the gateway
	incomingStream := [][]byte{
		{0x01, 0x64, 0x0A, 0xF0}, // Engine Data
		{0x02, 0x12, 0x34},       // Battery Data
		{0x03, 0xFF},             // Unknown Data
	}

	fmt.Println("🛰️  OmniBridge Gateway Active - Ingesting Stream...")

	for _, raw := range incomingStream {
		result, proto, err := dispatcher.Ingest(raw)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}
		fmt.Printf("✅ Identified [%s]: %v\n", proto, result)
	}
}
