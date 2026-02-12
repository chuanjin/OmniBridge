package parser

import (
	"fmt" // Keep fmt as it's used
	"io"
	"net"
	"time"

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

			// Attempt to run discovery synchronously for this connection
			// This blocks this specific client but ensures the first packet is not dropped.
			if s.discovery.IsDiscovering(sig) {
				logger.Info("Discovery already in progress, waiting...", zap.String("signature", sigHex))
				// In a real implementation, we might want a condition variable or a loop here.
				// For now, we'll just wait a bit and retry ingest, or drop if it takes too long.
				time.Sleep(2 * time.Second)
			} else {
				logger.Info("Unknown signature, starting BLOCKING AI discovery", zap.String("signature", sigHex))
				context := "Remote incoming binary data stream."
				newName, discErr := s.discovery.DiscoverNewProtocol(raw, sig, context)
				if discErr != nil {
					logger.Error("Discovery failed", zap.String("signature", sigHex), zap.Error(discErr))
					continue
				}
				logger.Info("Discovery Success: New Protocol Learned", zap.String("protocol", newName))
			}

			// Re-attempt ingestion after discovery
			result, proto, err = s.dispatcher.Ingest(raw)
			if err != nil {
				// If it still fails, then we really can't handle it
				logger.Error("Still unable to parse after discovery", zap.Error(err))
			}
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
