package main

import (
	"fmt"

	"github.com/chuanjin/OmniBridge/internal/parser"
)

func main() {
	engine := parser.NewEngine()

	// 1. Imagine this string came directly from your Golden Prompt via an API call
	aiGeneratedCode := `
package dynamic

func Parse(data []byte) map[string]interface{} {
	if len(data) < 2 { return nil }
	// Example: Byte 0 is Temp, Byte 1 is Humidity
	return map[string]interface{}{
		"temperature": float64(data[0]) * 0.5, 
		"humidity":    int(data[1]),
		"source":      "AI-Generated-Parser",
	}
}`

	// 2. Raw data from hardware (e.g., 0x28 is 40 in decimal)
	rawHardwareData := []byte{0x28, 0x50}

	// 3. Execute instantly
	result, err := engine.Execute(rawHardwareData, aiGeneratedCode)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Successfully Parsed via Dynamic AI Logic:")
	for k, v := range result {
		fmt.Printf("-> %s: %v\n", k, v)
	}
}
