package parser

// Result is what the AI should always return
type Result struct {
	Metrics map[string]interface{} `json:"metrics"`
	Error   string                 `json:"error,omitempty"`
}

// The AI-generated code will be expected to implement this logic
const Template = `
package dynamic

func Parse(data []byte) map[string]interface{} {
    // AI generated logic goes here
    return nil
}
`
