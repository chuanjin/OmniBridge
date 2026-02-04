package parser

import (
	"fmt"
	"io"
	"log"
	"net"
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
	defer listener.Close()

	log.Printf("📡 TCP Server listening on %s", s.addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("❌ Accept error: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *TCPServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("🔌 New connection from %s", conn.RemoteAddr())

	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("❌ Read error: %v", err)
			}
			break
		}

		raw := buffer[:n]
		log.Printf("📥 Received raw data (0x%X) from %s", raw, conn.RemoteAddr())

		// Attempt to parse using cached/known logic
		result, proto, err := s.dispatcher.Ingest(raw)

		// 1. SELF-HEALING: If ingest fails for a KNOWN protocol (e.g., compile error), try to repair it
		if err != nil && proto != "" {
			log.Printf("🔧 Detected error in [0x%X] (%s): %v. Attempting repair...", raw[0], proto, err)

			faultyCode, exists := s.dispatcher.GetManager().GetParserCode(proto)
			if exists {
				_, repairErr := s.discovery.RepairParser(proto, faultyCode, err.Error(), raw, nil)
				if repairErr != nil {
					log.Printf("❌ Repair failed: %v", repairErr)
				} else {
					// Re-attempt ingestion after repair
					result, proto, err = s.dispatcher.Ingest(raw)
					if err == nil {
						log.Printf("✨ Protocol %s repaired successfully!", proto)
					}
				}
			}
		}

		// 2. DISCOVERY: If protocol is entirely unknown
		if err != nil && proto == "" {
			log.Printf("🔍 Unknown signature [0x%X]. Consulting AI for discovery...", raw[0])

			// In a real server, we might want to pass more context hints if available
			context := "Remote incoming binary data stream."
			newName, discErr := s.discovery.DiscoverNewProtocol(raw, nil, context)

			if discErr != nil {
				log.Printf("❌ Discovery failed: %v", discErr)
				continue
			}

			// Re-attempt Ingestion
			result, proto, err = s.dispatcher.Ingest(raw)
			if err == nil {
				log.Printf("✨ New Protocol Learned & Persistent: %s", newName)
			}
		}

		if err == nil {
			log.Printf("✅ [SUCCESS] Protocol: %-15s | Data: %v", proto, result)
			// Optionally send result back to client or log it
			fmt.Fprintf(conn, "Parsed (%s): %v\n", proto, result)
		} else {
			fmt.Fprintf(conn, "Error: %v\n", err)
		}
	}
	log.Printf("🔌 Connection closed from %s", conn.RemoteAddr())
}
