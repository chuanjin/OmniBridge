package dynamic

// Signature: 01
func Parse(data []byte) map[string]interface{} {
	if len(data) < 2 {
		return nil
	}
	return map[string]interface{}{"rpm": int(data[1]) * 100}
}
