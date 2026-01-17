package main

import (
	"fmt"

	"github.com/chuanjin/OmniBridge/internal/parser"
)

func main() {
	mgr := parser.NewParserManager("./storage")
	mgr.LoadSavedParsers()
	dispatcher := parser.NewDispatcher(mgr)
	discovery := parser.NewDiscoveryService(dispatcher, mgr)

	// Known protocol
	dispatcher.Bind(0x01, "Engine_System")

	// Stream with an unknown signature (0x99)
	incomingStream := [][]byte{
		{0x01, 0x64}, // Known
		{0x99, 0x42}, // UNKNOWN - This will trigger Discovery
	}

	for _, raw := range incomingStream {
		result, proto, err := dispatcher.Ingest(raw)
		if err != nil {
			fmt.Println("🔍 Unknown protocol detected. Triggering Discovery Mode...")

			// Trigger AI to learn this new protocol
			newName, discErr := discovery.DiscoverNewProtocol(raw, "Industrial Sensor")
			if discErr != nil {
				fmt.Println("❌ Discovery failed:", discErr)
				continue
			}

			// Try parsing again now that it's learned
			result, proto, _ = dispatcher.Ingest(raw)
			fmt.Printf("✨ New Protocol Learned: %s\n", newName)
		}

		fmt.Printf("✅ Parsed [%s]: %v\n", proto, result)
	}
}
