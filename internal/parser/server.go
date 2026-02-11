package parser

import (
	"fmt"
	"io"
	"net"

	"github.com/chuanjin/OmniBridge/internal/logger"
	"go.uber.org/zap"
)

// TCPServer listens for incoming binary data streams
type TCPServer struct {
	addr       string
	dispatcher *Dispatcher
	discovery  *DiscoveryService
}

func NewTCPServer(addr string, d *Dispatcher, disc *DiscoveryService) *TCPServer {
	return &TCPServer{
		addr:       addr,
		dispatcher: d,
		discovery:  disc,
	}
}

func (s *TCPServer) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", s.addr, err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			logger.Error("Failed to close listener", zap.Error(err))
		}
	}()

	logger.Info("TCP Server listening", zap.String("address", s.addr))

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("Accept error", zap.Error(err))
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *TCPServer) handleConnection(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Error("Failed to close connection", zap.Error(err))
		}
	}()
	logger.Info("New connection", zap.String("remote_addr", conn.RemoteAddr().String()))

	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				logger.Error("Read error", zap.Error(err))
			}
			break
		}

		raw := buffer[:n]
		logger.Debug("Received raw data", zap.String("hex", fmt.Sprintf("0x%X", raw)), zap.String("remote_addr", conn.RemoteAddr().String()))

		// Attempt to parse using cached/known logic
		result, proto, err := s.dispatcher.Ingest(raw)

		// 1. SELF-HEALING: If ingest fails for a KNOWN protocol (e.g., compile error), try to repair it
		if err != nil && proto != "" {
			logger.Warn("Detected error in protocol", zap.String("protocol", proto), zap.Error(err))
			logger.Info("Attempting repair...")

			faultyCode, exists := s.dispatcher.GetManager().GetParserCode(proto)
			if exists {
				_, repairErr := s.discovery.RepairParser(proto, faultyCode, err.Error(), raw, nil)
				if repairErr != nil {
					logger.Error("Repair failed", zap.Error(repairErr))
				} else {
					// Re-attempt ingestion after repair
					result, proto, err = s.dispatcher.Ingest(raw)
					if err == nil {
						logger.Info("Protocol repaired successfully", zap.String("protocol", proto))
					}
				}
			}
		}

		// 2. DISCOVERY: If protocol is entirely unknown
		if err != nil && proto == "" {
			// Extract a tentative signature (e.g. first byte) to key the discovery process
			sig := []byte{raw[0]}
			sigHex := fmt.Sprintf("0x%X", sig)

			if s.discovery.StartDiscovery(sig) {
				logger.Info("Unknown signature, starting BACKGROUND AI discovery", zap.String("signature", sigHex))

				// Launch async discovery
				go func(sample []byte, signature []byte) {
					defer s.discovery.FinishDiscovery(signature)

					context := "Remote incoming binary data stream."
					newName, discErr := s.discovery.DiscoverNewProtocol(sample, signature, context)

					if discErr != nil {
						logger.Error("Async Discovery failed", zap.String("signature", sigHex), zap.Error(discErr))
						return
					}
					logger.Info("Async Discovery: New Protocol Learned", zap.String("protocol", newName))
				}(append([]byte(nil), raw...), sig) // Copy raw to avoid race conditions with buffer reuse

			} else {
				logger.Debug("Discovery already in progress", zap.String("signature", sigHex))
			}

			// Do NOT re-attempt ingestion here to avoid blocking.
			// The packet is effectively dropped or "skipped" until the parser is ready.
			continue
		}

		if err == nil {
			logger.Info("Success", zap.String("protocol", proto), zap.Any("data", result))
			// Optionally send result back to client or log it
			_, _ = fmt.Fprintf(conn, "Parsed (%s): %v\n", proto, result)
		} else {
			_, _ = fmt.Fprintf(conn, "Error: %v\n", err)
		}
	}
	logger.Info("Connection closed", zap.String("remote_addr", conn.RemoteAddr().String()))
}
