package dynamic

// Signature: 410C
func Parse(data []byte) map[string]interface{} {
	// OBD-II Response for Service 01 PID 0C (Engine RPM)
	// Format: 41 0C A B
	// Formula: ((A * 256) + B) / 4
	if len(data) < 4 {
		return nil
	}

	// data[0] is 0x41, data[1] is 0x0C
	a := int(data[2])
	b := int(data[3])
	rpm := (a*256 + b) / 4

	return map[string]interface{}{
		"rpm": rpm,
	}
}
