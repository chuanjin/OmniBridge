# 🌉 OmniBridge

OmniBridge is a dynamic binary protocol gateway that leverages Large Language Models (LLMs) to automatically discover and parse unknown data signatures in real-time. It provides a "Fast Path" for known protocols using high-performance Go-based parsers and a "Discovery Mode" for learning new protocols on the fly.

## 🚀 Key Features

- **Dynamic Protocol Discovery**: Automatically detects unknown binary signatures and consults an LLM to generate parsing logic.
- **Native Speed Execution**: LLM-generated Go code is executed via the `yaegi` interpreter at near-native speeds.
- **Persistence**: Learned protocols are saved to disk and restored automatically on startup.
- **Multi-LLM Support**: Supports local models via **Ollama** and cloud models via **Google Gemini**.
- **Context-Aware Parsing**: Uses system prompts and data hints to improve the accuracy of generated parsers.

## 🏗️ Architecture

OmniBridge consists of several key components:

- **Dispatcher**: Routes incoming data streams based on their first-byte signature.
- **Parser Manager**: Manages the lifecycle of parsers, including registration, storage, and execution.
- **Discovery Service**: The bridge to LLMs, responsible for generating new Go code when an unknown signature is encountered.
- **Engine**: A high-performance execution environment for dynamic Go code.

### The Lifecycle of a Packet

1. **Ingest**: A raw byte stream arrives.
2. **Dispatch**: The Dispatcher checks if the signature (first byte) is known.
3. **Fast Path (Known)**: If known, the pre-compiled or previously learned parser is executed immediately.
4. **Discovery Mode (Unknown)**:
   - The Discovery Service sends the raw sample and context hints to the configured LLM.
   - The LLM generates a valid Go `Parse` function.
   - The code is sanitized, saved to `./storage`, and bound to the signature.
   - The packet is re-ingested using the new parser.

## 🛠️ Getting Started

### Prerequisites

- **Go**: Version 1.25.5 or later.
- **LLM Access**: 
  - For **Gemini**: A valid `GEMINI_API_KEY` environment variable.
  - For **Ollama**: A running Ollama instance with the desired model (e.g., `deepseek-coder:1.3b`).

### Installation

```bash
git clone https://github.com/chuanjin/OmniBridge.git
cd OmniBridge
go mod tidy
```

### Configuration

Create a `.env` file in the root directory:

```env
GEMINI_API_KEY=your_api_key_here
```

### Running the Server

Start the OmniBridge gateway with CLI flags:

```bash
go run cmd/server/main.go --provider gemini --model gemini-2.0-flash
```

Or using Ollama:

```bash
go run cmd/server/main.go --provider ollama --model deepseek-coder:1.3b
```

## 📁 Project Structure

- `cmd/server/`: Main application entry point.
- `internal/parser/`: Core logic for dispatching, managing, and discovering protocols.
- `internal/mcp/`: Model Context Protocol (MCP) handlers.
- `agents/`: System prompts for the LLM.
- `storage/`: Persistent storage for generated Go parsers and the manifest.
- `examples/`: Sample usage and data streams.

## 📜 License

This project is licensed under the MIT License - see the `LICENSE` file for details.
