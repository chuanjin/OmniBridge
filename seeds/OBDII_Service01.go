//go:build ignore

package dynamic

import (
	"fmt"
)

// Signature: 41
func Parse(data []byte) map[string]interface{} {
	// OBD-II Response for Service 01 (Show current data)
	// Format: 41 PID A B C D ...
	if len(data) < 2 {
		return nil
	}

	pid := data[1]
	res := map[string]interface{}{
		"pid": fmt.Sprintf("%02X", pid),
	}

	// Helper function for common byte patterns
	getVal := func(idx int) float64 {
		if idx+2 < len(data) {
			return float64(data[idx+2])
		}
		return 0
	}

	switch pid {
	case 0x01:
		res["name"] = "Monitor status since DTCs cleared"
		// Bit-encoded data
	case 0x04:
		res["name"] = "Calculated engine load"
		res["value"] = int(getVal(0) * 100 / 255)
		res["unit"] = "%"
	case 0x05:
		res["name"] = "Engine coolant temperature"
		res["value"] = getVal(0) - 40
		res["unit"] = "°C"
	case 0x0B:
		res["name"] = "Intake manifold absolute pressure"
		res["value"] = getVal(0)
		res["unit"] = "kPa"
	case 0x0C:
		res["name"] = "Engine speed"
		res["value"] = (getVal(0)*256 + getVal(1)) / 4
		res["unit"] = "rpm"
	case 0x0D:
		res["name"] = "Vehicle speed"
		res["value"] = getVal(0)
		res["unit"] = "km/h"
	case 0x0F:
		res["name"] = "Intake air temperature"
		res["value"] = getVal(0) - 40
		res["unit"] = "°C"
	case 0x10:
		res["name"] = "MAF air flow rate"
		res["value"] = (getVal(0)*256 + getVal(1)) / 100
		res["unit"] = "g/s"
	case 0x11:
		res["name"] = "Throttle position"
		res["value"] = int(getVal(0) * 100 / 255)
		res["unit"] = "%"
	case 0x1F:
		res["name"] = "Run time since engine start"
		res["value"] = getVal(0)*256 + getVal(1)
		res["unit"] = "s"
	case 0x21:
		res["name"] = "Distance traveled with MIL on"
		res["value"] = getVal(0)*256 + getVal(1)
		res["unit"] = "km"
	case 0x2F:
		res["name"] = "Fuel Tank Level Input"
		res["value"] = int(getVal(0) * 100 / 255)
		res["unit"] = "%"
	case 0x33:
		res["name"] = "Absolute Barometric Pressure"
		res["value"] = getVal(0)
		res["unit"] = "kPa"
	case 0x42:
		res["name"] = "Control module voltage"
		res["value"] = (getVal(0)*256 + getVal(1)) / 1000
		res["unit"] = "V"
	case 0x5C:
		res["name"] = "Engine oil temperature"
		res["value"] = getVal(0) - 40
		res["unit"] = "°C"
	default:
		res["name"] = "Unknown Service 01 PID"
		res["raw_data"] = data[2:]
	}

	return res
}
