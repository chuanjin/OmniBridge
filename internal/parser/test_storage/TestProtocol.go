
package dynamic

func Parse(data []byte) map[string]interface{} {
	return map[string]interface{}{
		"value": int(data[1]),
	}
}
