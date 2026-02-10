package parser

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sync"
)

type Dispatcher struct {
	manager *ParserManager
	// Map of Hex Signature Prefix -> ProtocolID (e.g., "01" -> "VolvoEngine", "012A" -> "SpecialSensor")
	routes map[string]string
	mu     sync.RWMutex
}

// GetBindings returns a copy of the current signature-to-parser mappings.
func (d *Dispatcher) GetBindings() map[string]string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	copy := make(map[string]string)
	for k, v := range d.routes {
		copy[k] = v
	}

	return copy
}

func (d *Dispatcher) GetManager() *ParserManager {
	return d.manager
}

func NewDispatcher(mgr *ParserManager) *Dispatcher {
	return &Dispatcher{
		manager: mgr,
		routes:  make(map[string]string),
	}
}

// Bind links a specific byte slice (signature) to a parser
func (d *Dispatcher) Bind(signature []byte, protocolID string) {
	hexSig := fmt.Sprintf("%X", signature)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.routes[hexSig] = protocolID
}

// Ingest takes raw data, identifies the protocol, and parses it
func (d *Dispatcher) Ingest(data []byte) (map[string]interface{}, string, error) {
	if len(data) == 0 {
		return nil, "", fmt.Errorf("empty payload")
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var matchedProto string
	var matchedSig string

	// Longest prefix match
	for sig, proto := range d.routes {
		sigBytes := hexToBytes(sig)
		if bytes.HasPrefix(data, sigBytes) {
			if len(sig) > len(matchedSig) {
				matchedSig = sig
				matchedProto = proto
			}
		}
	}

	if matchedProto == "" {
		maxLen := 4
		if len(data) < maxLen {
			maxLen = len(data)
		}
		return nil, "", fmt.Errorf("unknown protocol signature: 0x%X", data[:maxLen])
	}

	// Use the manager to run the cached parser
	result, err := d.manager.ParseData(matchedProto, data)
	return result, matchedProto, err
}

func hexToBytes(h string) []byte {
	if len(h)%2 != 0 {
		h = "0" + h
	}
	b, _ := hex.DecodeString(h)
	return b
}
