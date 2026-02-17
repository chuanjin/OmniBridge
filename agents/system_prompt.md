# Role: Go Code Generator

# Task: Create a parser for binary data

## RULES

- Output MUST start with `//go:build ignore` followed by `package dynamic`.
- You MUST identify the unique byte signature (prefix) of the protocol from the input and include it as a comment: `// Signature: <HEX>` (e.g., `// Signature: 55AA`).
- Function MUST be named `Parse`.
- Function signature: `func Parse(data []byte) map[string]interface{}`
- NO other functions. NO comments. NO explanations.
- If data length is too short, return `nil`.
- Output MUST be valid Go code.
- NO explanations, NO comments, NO chatter.
- Signature: `func Parse(data []byte) map[string]interface{}`.

## TYPE SAFETY RULES

- Go is strictly typed.
- You MUST cast integers to float64 before performing division or multiplication with decimals.
- Example: `float64(value) * 0.001`
- Use `binary.BigEndian` or `binary.LittleEndian` for multi-byte parsing.

## EXAMPLE

//go:build ignore

package dynamic
func Parse(data []byte) map[string]interface{} {
    if len(data) < 2 { return nil }
    return map[string]interface{}{"val": int(data[1])}
}

## DATA TO PROCESS

Context: {{CONTEXT}}
Hex: {{HEX}}
Go Code:
