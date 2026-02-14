package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/chuanjin/OmniBridge/internal/parser"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPServerInitialization(t *testing.T) {
	// Setup
	mgr := parser.NewParserManager("./test_storage", "")
	dispatcher := parser.NewDispatcher(mgr)
	cfg := parser.DiscoveryConfig{
		Provider: "ollama",
		Model:    "test-model",
		Endpoint: "http://localhost:11434/api/generate",
	}
	discovery := parser.NewDiscoveryService(dispatcher, mgr, cfg)

	// Create MCP server
	server := NewServer(dispatcher, mgr, discovery)

	// Verify server is created
	assert.NotNil(t, server)
	assert.NotNil(t, server.mcpServer)
	assert.Equal(t, dispatcher, server.dispatcher)
	assert.Equal(t, mgr, server.manager)
	assert.Equal(t, discovery, server.discovery)
}

func TestListProtocolsHandler(t *testing.T) {
	// Setup
	mgr := parser.NewParserManager("./test_storage", "")
	dispatcher := parser.NewDispatcher(mgr)
	cfg := parser.DiscoveryConfig{
		Provider: "ollama",
		Model:    "test-model",
		Endpoint: "http://localhost:11434/api/generate",
	}
	discovery := parser.NewDiscoveryService(dispatcher, mgr, cfg)

	// Add a test protocol binding
	dispatcher.Bind([]byte{0x01}, "test_protocol")

	server := NewServer(dispatcher, mgr, discovery)

	// Create a mock request
	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	// Call the handler
	result, output, err := server.handleListProtocols(ctx, req, struct{}{})

	// Verify
	require.NoError(t, err)
	assert.Nil(t, result)
	assert.Len(t, output.Protocols, 1)
	assert.Equal(t, "test_protocol", output.Protocols[0].Name)
	assert.Equal(t, "01", output.Protocols[0].Signature)
}

func TestProtocolListResource(t *testing.T) {
	// Setup
	mgr := parser.NewParserManager("./test_storage", "")
	dispatcher := parser.NewDispatcher(mgr)
	cfg := parser.DiscoveryConfig{
		Provider: "ollama",
		Model:    "test-model",
		Endpoint: "http://localhost:11434/api/generate",
	}
	discovery := parser.NewDiscoveryService(dispatcher, mgr, cfg)

	// Add test protocol bindings
	dispatcher.Bind([]byte{0x01}, "protocol_one")
	dispatcher.Bind([]byte{0x02}, "protocol_two")

	server := NewServer(dispatcher, mgr, discovery)

	// Create a mock request
	ctx := context.Background()
	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "protocol://list",
		},
	}

	// Call the handler
	result, err := server.handleProtocolList(ctx, req)

	// Verify
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Contents, 1)
	assert.Equal(t, "protocol://list", result.Contents[0].URI)
	assert.Equal(t, "application/json", result.Contents[0].MIMEType)

	// Parse the JSON content
	var protocols []map[string]string
	err = json.Unmarshal([]byte(result.Contents[0].Text), &protocols)
	require.NoError(t, err)
	assert.Len(t, protocols, 2)
}

func TestManifestResource(t *testing.T) {
	// Setup
	mgr := parser.NewParserManager("./test_storage", "")
	dispatcher := parser.NewDispatcher(mgr)
	cfg := parser.DiscoveryConfig{
		Provider: "ollama",
		Model:    "test-model",
		Endpoint: "http://localhost:11434/api/generate",
	}
	discovery := parser.NewDiscoveryService(dispatcher, mgr, cfg)

	server := NewServer(dispatcher, mgr, discovery)

	// Create a mock request
	ctx := context.Background()
	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "protocol://manifest",
		},
	}

	// Call the handler
	result, err := server.handleManifest(ctx, req)

	// Verify
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Contents, 1)
	assert.Equal(t, "protocol://manifest", result.Contents[0].URI)
	assert.Equal(t, "application/json", result.Contents[0].MIMEType)

	// Parse the JSON content
	var manifest map[string]string
	err = json.Unmarshal([]byte(result.Contents[0].Text), &manifest)
	require.NoError(t, err)
}
