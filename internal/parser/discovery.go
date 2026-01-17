package parser

import (
	"fmt"
)

// DiscoveryService handles the interaction with the LLM when a protocol is unknown
type DiscoveryService struct {
	dispatcher *Dispatcher
	manager    *ParserManager
}

func NewDiscoveryService(d *Dispatcher, m *ParserManager) *DiscoveryService {
	return &DiscoveryService{dispatcher: d, manager: m}
}

// DiscoverNewProtocol simulates the call to an LLM (Claude/GPT)
func (s *DiscoveryService) DiscoverNewProtocol(rawSample []byte, context string) (string, error) {
	fmt.Printf("🕵️  Analyzing unknown signature: 0x%X...\n", rawSample[0])

	// --- In a real app, this is an API call to OpenAI/Anthropic ---
	// We pass the hex sample and the Golden Prompt.
	// For this demo, we simulate the AI identifying it as a "Humidity Sensor"
	simulatedProtocolName := fmt.Sprintf("Auto_Generated_0x%X", rawSample[0])
	simulatedAiCode := `
package dynamic
func Parse(data []byte) map[string]interface{} {
	return map[string]interface{}{
		"discovered_value": int(data[1]),
		"note": "Automatically identified by OmniBridge AI",
	}
}`

	// 1. Register the new parser in the manager (saves to disk/cache)
	err := s.manager.RegisterParser(simulatedProtocolName, simulatedAiCode)
	if err != nil {
		return "", err
	}

	// 2. Bind the signature in the dispatcher so it's recognized next time
	s.dispatcher.Bind(rawSample[0], simulatedProtocolName)

	return simulatedProtocolName, nil
}
