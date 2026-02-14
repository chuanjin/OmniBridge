package mcp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chuanjin/OmniBridge/internal/logger"
	"github.com/chuanjin/OmniBridge/internal/parser"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// Server wraps the MCP server with OmniBridge dependencies
type Server struct {
	dispatcher *parser.Dispatcher
	manager    *parser.ParserManager
	discovery  *parser.DiscoveryService
	mcpServer  *mcp.Server
}

// NewServer creates a new MCP server for OmniBridge
func NewServer(d *parser.Dispatcher, m *parser.ParserManager, disc *parser.DiscoveryService) *Server {
	s := &Server{
		dispatcher: d,
		manager:    m,
		discovery:  disc,
	}

	// Create MCP server with implementation info
	impl := &mcp.Implementation{
		Name:    "omnibridge",
		Version: "1.0.0",
	}

	s.mcpServer = mcp.NewServer(impl, nil)

	// Register resources, tools, and prompts
	s.registerResources()
	s.registerTools()
	s.registerPrompts()

	return s
}

// Run starts the MCP server over stdio transport
func (s *Server) Run(ctx context.Context) error {
	logger.Info("Starting OmniBridge MCP Server...")
	transport := &mcp.StdioTransport{}
	return s.mcpServer.Run(ctx, transport)
}

// registerResources adds all MCP resources
func (s *Server) registerResources() {
	// Resource: protocol://list - List all known protocols
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "protocol://list",
		Name:        "Protocol List",
		Description: "List of all known protocols with their signatures",
		MIMEType:    "application/json",
	}, s.handleProtocolList)

	// Resource: protocol://manifest - Get the complete manifest
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "protocol://manifest",
		Name:        "Protocol Manifest",
		Description: "Complete manifest mapping signatures to protocol parsers",
		MIMEType:    "application/json",
	}, s.handleManifest)
}

// registerTools adds all MCP tools
func (s *Server) registerTools() {
	// Tool: parse_binary - Parse binary data
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "parse_binary",
		Description: "Parse binary data using known protocol parsers",
	}, s.handleParseBinary)

	// Tool: discover_protocol - Discover new protocol
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "discover_protocol",
		Description: "Trigger AI-based protocol discovery for unknown binary data",
	}, s.handleDiscoverProtocol)

	// Tool: list_protocols - List all protocols
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_protocols",
		Description: "List all available protocol parsers",
	}, s.handleListProtocols)
}

// registerPrompts adds all MCP prompts
func (s *Server) registerPrompts() {
	// Prompt: protocol_discovery
	s.mcpServer.AddPrompt(&mcp.Prompt{
		Name:        "protocol_discovery",
		Description: "Template for discovering new binary protocols using AI",
	}, s.handleProtocolDiscoveryPrompt)

	// Prompt: parser_repair
	s.mcpServer.AddPrompt(&mcp.Prompt{
		Name:        "parser_repair",
		Description: "Template for repairing broken protocol parsers",
	}, s.handleParserRepairPrompt)
}

// Resource Handlers

func (s *Server) handleProtocolList(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	bindings := s.dispatcher.GetBindings()

	protocols := make([]map[string]string, 0, len(bindings))
	for sig, name := range bindings {
		protocols = append(protocols, map[string]string{
			"signature": sig,
			"name":      name,
		})
	}

	data, err := json.MarshalIndent(protocols, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func (s *Server) handleManifest(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	manifest, err := s.manager.LoadManifest()
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

// Tool Handlers

type ParseBinaryInput struct {
	Data string `json:"data" jsonschema:"Hex-encoded binary data to parse"`
}

type ParseBinaryOutput struct {
	Protocol string                 `json:"protocol" jsonschema:"Name of the protocol used to parse the data"`
	Result   map[string]interface{} `json:"result" jsonschema:"Parsed data structure"`
}

func (s *Server) handleParseBinary(ctx context.Context, req *mcp.CallToolRequest, input ParseBinaryInput) (*mcp.CallToolResult, ParseBinaryOutput, error) {
	// Decode hex string
	data, err := hex.DecodeString(input.Data)
	if err != nil {
		return nil, ParseBinaryOutput{}, fmt.Errorf("invalid hex data: %v", err)
	}

	// Attempt to parse
	result, proto, err := s.dispatcher.Ingest(data)
	if err != nil {
		return nil, ParseBinaryOutput{}, fmt.Errorf("parse failed: %v", err)
	}

	logger.Info("MCP: Parsed binary data", zap.String("protocol", proto))

	return nil, ParseBinaryOutput{
		Protocol: proto,
		Result:   result,
	}, nil
}

type DiscoverProtocolInput struct {
	Sample  string `json:"sample" jsonschema:"Hex-encoded binary sample data"`
	Context string `json:"context" jsonschema:"Optional context hint about the protocol"`
}

type DiscoverProtocolOutput struct {
	ProtocolName string `json:"protocol_name" jsonschema:"Name of the discovered protocol"`
	Signature    string `json:"signature" jsonschema:"Hex signature of the protocol"`
}

func (s *Server) handleDiscoverProtocol(ctx context.Context, req *mcp.CallToolRequest, input DiscoverProtocolInput) (*mcp.CallToolResult, DiscoverProtocolOutput, error) {
	// Decode hex string
	sample, err := hex.DecodeString(input.Sample)
	if err != nil {
		return nil, DiscoverProtocolOutput{}, fmt.Errorf("invalid hex sample: %v", err)
	}

	// Trigger discovery
	contextHint := input.Context
	if contextHint == "" {
		contextHint = "Unknown binary protocol"
	}

	logger.Info("MCP: Starting protocol discovery", zap.String("context", contextHint))

	protoName, err := s.discovery.DiscoverNewProtocol(sample, nil, contextHint)
	if err != nil {
		return nil, DiscoverProtocolOutput{}, fmt.Errorf("discovery failed: %v", err)
	}

	// Get the signature from bindings
	bindings := s.dispatcher.GetBindings()
	var signature string
	for sig, name := range bindings {
		if name == protoName {
			signature = sig
			break
		}
	}

	logger.Info("MCP: Protocol discovered", zap.String("protocol", protoName), zap.String("signature", signature))

	return nil, DiscoverProtocolOutput{
		ProtocolName: protoName,
		Signature:    signature,
	}, nil
}

type ListProtocolsOutput struct {
	Protocols []ProtocolInfo `json:"protocols" jsonschema:"List of available protocols"`
}

type ProtocolInfo struct {
	Name      string `json:"name" jsonschema:"Protocol name"`
	Signature string `json:"signature" jsonschema:"Hex signature"`
}

func (s *Server) handleListProtocols(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, ListProtocolsOutput, error) {
	bindings := s.dispatcher.GetBindings()

	protocols := make([]ProtocolInfo, 0, len(bindings))
	for sig, name := range bindings {
		protocols = append(protocols, ProtocolInfo{
			Name:      name,
			Signature: sig,
		})
	}

	logger.Info("MCP: Listed protocols", zap.Int("count", len(protocols)))

	return nil, ListProtocolsOutput{
		Protocols: protocols,
	}, nil
}

// Prompt Handlers

type ProtocolDiscoveryPromptArgs struct {
	SampleData  string `json:"sample_data" jsonschema:"Hex-encoded sample binary data"`
	ContextHint string `json:"context_hint" jsonschema:"Optional hint about the protocol"`
}

func (s *Server) handleProtocolDiscoveryPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	// Parse arguments
	var args ProtocolDiscoveryPromptArgs
	if req.Params.Arguments != nil {
		data, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &args); err != nil {
			return nil, err
		}
	}

	// Load system prompt
	absPath, _ := filepath.Abs("agents/system_prompt.md")
	systemPrompt, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load system_prompt.md: %v", err)
	}

	contextHint := args.ContextHint
	if contextHint == "" {
		contextHint = "Unknown binary protocol"
	}

	fullPrompt := fmt.Sprintf("%s\n\nINPUT:\nHex Sample: %s\nProtocol Hints: %s",
		string(systemPrompt), args.SampleData, contextHint)

	return &mcp.GetPromptResult{
		Description: "Protocol discovery prompt for AI-based binary protocol analysis",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: fullPrompt,
				},
			},
		},
	}, nil
}

type ParserRepairPromptArgs struct {
	ProtocolName string `json:"protocol_name" jsonschema:"Name of the protocol to repair"`
	ErrorMessage string `json:"error_message" jsonschema:"Error message from the failed parser"`
	SampleData   string `json:"sample_data" jsonschema:"Hex-encoded sample data that failed to parse"`
}

func (s *Server) handleParserRepairPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	// Parse arguments
	var args ParserRepairPromptArgs
	if req.Params.Arguments != nil {
		data, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &args); err != nil {
			return nil, err
		}
	}

	// Load system prompt
	absPath, _ := filepath.Abs("agents/system_prompt.md")
	systemPrompt, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load system_prompt.md: %v", err)
	}

	// Get faulty code
	faultyCode, exists := s.manager.GetParserCode(args.ProtocolName)
	if !exists {
		return nil, fmt.Errorf("protocol %s not found", args.ProtocolName)
	}

	fullPrompt := fmt.Sprintf("%s\n\n### ERROR TO FIX\nYou previously generated code that failed.\n\nFAULTY CODE:\n```go\n%s\n```\n\nERROR MESSAGE:\n%s\n\nINPUT DATA (Hex): %s\n\nPlease fix the code and return only the valid Go code.",
		string(systemPrompt), faultyCode, args.ErrorMessage, args.SampleData)

	return &mcp.GetPromptResult{
		Description: "Parser repair prompt for fixing broken protocol parsers",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: fullPrompt,
				},
			},
		},
	}, nil
}
