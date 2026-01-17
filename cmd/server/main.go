package main

import (
	"fmt"

	"github.com/chuanjin/OmniBridge/internal/parser"
)

func main() {
	// Initialize Manager with a storage folder
	mgr := parser.NewParserManager("./storage")
	mgr.LoadSavedParsers()

	protocolID := "CAN_V1_Engine"

	// --- PHASE 1: THE LEARNING (Slow, only done once) ---
	// In a real scenario, this 'aiCode' comes from an LLM API call
	aiCode := `
package dynamic
func Parse(data []byte) map[string]interface{} {
	return map[string]interface{}{"rpm": int(data[0]) * 100, "temp": int(data[1])}
}`
	mgr.RegisterParser(protocolID, aiCode)

	// --- PHASE 2: THE DATA PLANE (Fast, repeated millions of times) ---
	// High-speed data ingestion simulation
	rawBusData := []byte{0x1E, 0x5A} // 3000 RPM, 90 Degrees

	result, err := mgr.ParseData(protocolID, rawBusData)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("\n🚀 High-Speed Parse Result for %s:\n", protocolID)
	fmt.Printf("RPM: %v, Temp: %v\n", result["rpm"], result["temp"])
}
