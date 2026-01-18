package parser

import (
	"fmt"
	"sync"
)

type Dispatcher struct {
	manager *ParserManager
	// Map of Byte Signature -> ProtocolID (e.g., "0x05" -> "VolvoEngine")
	routes map[byte]string
	mu     sync.RWMutex
}

// GetBindings returns a copy of the current signature-to-parser mappings.
// This is used by the DiscoveryService to persist learned protocols to disk.
func (d *Dispatcher) GetBindings() map[uint8]string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Create a copy to ensure thread safety and data integrity
	copy := make(map[uint8]string)
	for k, v := range d.routes {
		copy[k] = v
	}

	return copy
}

func NewDispatcher(mgr *ParserManager) *Dispatcher {
	return &Dispatcher{
		manager: mgr,
		routes:  make(map[byte]string),
	}
}

// Bind links a specific leading byte (signature) to a parser
func (d *Dispatcher) Bind(signature byte, protocolID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.routes[signature] = protocolID
}

// Ingest takes raw data, identifies the protocol, and parses it
func (d *Dispatcher) Ingest(data []byte) (map[string]interface{}, string, error) {
	if len(data) == 0 {
		return nil, "", fmt.Errorf("empty payload")
	}

	d.mu.RLock()
	protocolID, exists := d.routes[data[0]] // Peek at the first byte
	d.mu.RUnlock()

	if !exists {
		return nil, "", fmt.Errorf("unknown protocol signature: 0x%X", data[0])
	}

	// Use the manager to run the cached parser
	result, err := d.manager.ParseData(protocolID, data)
	return result, protocolID, err
}
