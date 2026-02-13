# 🌉 OmniBridge

[![CI](https://github.com/chuanjin/OmniBridge/actions/workflows/ci.yml/badge.svg)](https://github.com/chuanjin/OmniBridge/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

**OmniBridge is an AI-native binary protocol gateway**: it parses known protocols at high speed and **learns unknown ones automatically** using an LLM, then persists the learned parser for future traffic.

> If your data streams are evolving faster than hand-written decoders, OmniBridge gives you a practical way to keep up.

---

## ✨ Why OmniBridge

- ⚡ **Fast path first**: Known signatures route directly to existing parsers.
- 🧠 **AI discovery mode**: Unknown packets trigger LLM-assisted parser generation.
- 🔁 **Self-healing parsers**: If a learned parser fails at runtime, OmniBridge attempts automatic repair.
- 💾 **Persistent learning**: Generated parsers are saved in `./storage` and reloaded on startup.
- 🔌 **Provider flexibility**: Works with **Gemini** (cloud) and **Ollama** (local).
- 🧪 **Tested core**: Includes unit tests for discovery, dispatcher, manager, and execution engine.

---

## 🏗️ How it works

1. **Ingest** raw bytes from simulation mode or TCP server mode.
2. **Dispatch** by signature (supports multi-byte signatures).
3. **Parse on fast path** when a parser already exists.
4. **Discover on miss** by asking the configured LLM to generate a Go `Parse` function.
5. **Bind + persist** generated parser, then re-ingest the packet.
6. **Repair automatically** if a known parser breaks on future packets.

---

## 🚀 Quick Start

### 1) Prerequisites

- Go **1.25+**
- One LLM provider:
  - **Gemini**: set `GEMINI_API_KEY`
  - **Ollama**: local Ollama server running

### 2) Install

```bash
git clone https://github.com/chuanjin/OmniBridge.git
cd OmniBridge
go mod tidy
```

### 3) Configure environment

Create a `.env` file:

```env
# Needed only for Gemini provider
GEMINI_API_KEY=your_api_key_here
```

### 4) Run in simulation mode (default)

```bash
go run cmd/server/main.go --provider gemini --model gemini-2.0-flash
```

Run with Ollama:

```bash
go run cmd/server/main.go --provider ollama --model deepseek-coder:1.3b
```

### 5) Run as TCP gateway

```bash
go run cmd/server/main.go --mode server --addr :8080 --provider gemini --model gemini-2.0-flash
```

Send binary data to it from your client; OmniBridge will parse known signatures and discover unknown ones.

---

## 🐳 Docker

Build and run:

```bash
docker build -t omnibridge .
docker run --rm -p 8080:8080 --env GEMINI_API_KEY=$GEMINI_API_KEY omnibridge
```

---

## 📁 Project layout

- `cmd/server/` — CLI entrypoint (simulation + TCP server modes)
- `internal/parser/` — dispatcher, discovery service, parser manager, dynamic engine
- `internal/logger/` — structured logging setup
- `agents/` — system prompt(s) used for parser generation
- `seeds/` — built-in parser seeds loaded at startup
- `examples/` — sample protocol data
- `storage/` — learned parsers + manifest (created at runtime)

---

## 🧭 Current project status

OmniBridge is actively evolving and already usable for experimentation/prototyping with mixed known/unknown binary streams.

High-impact areas underway:

- Better parser validation and safety hardening
- Expanded seeded protocol coverage
- More production-oriented observability and deployment patterns

If this roadmap aligns with your use case, a ⭐ helps prioritize development.

---

## 🤝 Contributing

Issues and PRs are welcome. If you have a target protocol family (CAN, telemetry, industrial buses, custom IoT frames), open an issue with sample payloads and expected fields.

---

## 📜 License

MIT — see [`LICENSE`](./LICENSE).
