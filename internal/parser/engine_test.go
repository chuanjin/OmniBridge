package parser

import (
	"fmt"
	"reflect"
	"testing"
)

func TestEngine_Execute_UniversalService01(t *testing.T) {
	e := NewEngine()

	// The Universal Parser code (simulated here for tests)
	universalCode := `package dynamic
import "fmt"
func Parse(data []byte) map[string]interface{} {
	if len(data) < 2 { return nil }
	pid := data[1]
	getVal := func(idx int) float64 {
		if idx+2 < len(data) { return float64(data[idx+2]) }
		return 0
	}
	res := map[string]interface{}{"pid": fmt.Sprintf("%02X", pid)}
	switch pid {
	case 0x04:
		res["name"] = "Calculated engine load"; res["value"] = int(getVal(0) * 100 / 255); res["unit"] = "%"
	case 0x05:
		res["name"] = "Engine coolant temperature"; res["value"] = getVal(0) - 40; res["unit"] = "°C"
	case 0x0C:
		res["name"] = "Engine speed"; res["value"] = (getVal(0)*256 + getVal(1)) / 4; res["unit"] = "rpm"
	case 0x0D:
		res["name"] = "Vehicle speed"; res["value"] = getVal(0); res["unit"] = "km/h"
	case 0x10:
		res["name"] = "MAF air flow rate"; res["value"] = (getVal(0)*256 + getVal(1)) / 100; res["unit"] = "g/s"
	case 0x21:
		res["name"] = "Distance traveled with MIL on"; res["value"] = getVal(0)*256 + getVal(1); res["unit"] = "km"
	}
	return res
}`

	tests := []struct {
		name     string
		rawData  []byte
		expected map[string]interface{}
	}{
		{
			name:    "PID 04: Engine Load",
			rawData: []byte{0x41, 0x04, 0x7F},
			expected: map[string]interface{}{
				"pid": "04", "name": "Calculated engine load", "value": 49, "unit": "%",
			},
		},
		{
			name:    "PID 0C: Engine Speed",
			rawData: []byte{0x41, 0x0C, 0x1A, 0xF8},
			expected: map[string]interface{}{
				"pid": "0C", "name": "Engine speed", "value": 1726.0, "unit": "rpm",
			},
		},
		{
			name:    "PID 0D: Vehicle Speed",
			rawData: []byte{0x41, 0x0D, 0x64},
			expected: map[string]interface{}{
				"pid": "0D", "name": "Vehicle speed", "value": 100.0, "unit": "km/h",
			},
		},
		{
			name:    "PID 21: MIL Distance",
			rawData: []byte{0x41, 0x21, 0x04, 0xD2},
			expected: map[string]interface{}{
				"pid": "21", "name": "Distance traveled with MIL on", "value": 1234.0, "unit": "km",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Execute("universal_test", tt.rawData, universalCode)
			if err != nil {
				t.Errorf("Engine.Execute() error = %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Engine.Execute() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEngine_Execute_Sandboxing(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{
			name: "Forbidden import: os",
			code: `package dynamic
import "os"
func Parse(data []byte) map[string]interface{} {
	_ = os.Args
	return nil
}`,
			wantErr: true,
		},
		{
			name: "Allowed import: fmt",
			code: `package dynamic
import "fmt"
func Parse(data []byte) map[string]interface{} {
	_ = fmt.Sprintf("test")
	return nil
}`,
			wantErr: false,
		},
		{
			name: "Forbidden import: net",
			code: `package dynamic
import "net"
func Parse(data []byte) map[string]interface{} {
	_, _ = net.Dial("tcp", "localhost:80")
	return nil
}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.Execute(tt.name, []byte{0x00}, tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("Engine.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkExecute_Uncached(b *testing.B) {
	e := NewEngine()
	code := `package dynamic
func Parse(data []byte) map[string]interface{} {
	return map[string]interface{}{"val": data[0]}
}`
	data := []byte{0x01}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Using a different ID every time to force compilation
		_, _ = e.Execute(fmt.Sprintf("id_%d", i), data, code)
	}
}

func BenchmarkExecute_Cached(b *testing.B) {
	e := NewEngine()
	code := `package dynamic
func Parse(data []byte) map[string]interface{} {
	return map[string]interface{}{"val": data[0]}
}`
	data := []byte{0x01}
	_, _ = e.Execute("fixed_id", data, code) // Pre-cache
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Using the same ID to use cache
		_, _ = e.Execute("fixed_id", data, code)
	}
}
