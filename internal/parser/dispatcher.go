package parser

import (
	"encoding/hex"
	"fmt"
	"sync"
)

type trieNode struct {
	children   map[byte]*trieNode
	protocolID string
}

type Dispatcher struct {
	manager *ParserManager
	// Map of Hex Signature Prefix -> ProtocolID (e.g., "01" -> "VolvoEngine", "012A" -> "SpecialSensor")
	routes map[string]string
	root   *trieNode
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
		root:    &trieNode{children: make(map[byte]*trieNode)},
	}
}

// Bind links a specific byte slice (signature) to a parser
func (d *Dispatcher) Bind(signature []byte, protocolID string) {
	hexSig := fmt.Sprintf("%X", signature)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.routes[hexSig] = protocolID

	// Insert into Trie
	curr := d.root
	for _, b := range signature {
		if curr.children == nil {
			curr.children = make(map[byte]*trieNode)
		}
		if _, ok := curr.children[b]; !ok {
			curr.children[b] = &trieNode{children: make(map[byte]*trieNode)}
		}
		curr = curr.children[b]
	}
	curr.protocolID = protocolID
}

// Ingest takes raw data, identifies the protocol, and parses it
func (d *Dispatcher) Ingest(data []byte) (map[string]interface{}, string, error) {
	if len(data) == 0 {
		return nil, "", fmt.Errorf("empty payload")
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var matchedProto string
	curr := d.root

	// Longest prefix match using Trie
	for _, b := range data {
		if next, ok := curr.children[b]; ok {
			curr = next
			if curr.protocolID != "" {
				matchedProto = curr.protocolID
			}
		} else {
			break
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
