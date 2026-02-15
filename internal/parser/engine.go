package parser

import (
	"fmt"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// symbols defines the restricted set of standard library symbols available to parsers
var symbols = make(interp.Exports)

func init() {
	// Only provide symbols necessary for binary parsing
	for pkg, export := range stdlib.Symbols {
		switch pkg {
		case "fmt/fmt", "encoding/binary/binary", "math/math", "math/bits/bits", "bytes/bytes", "strconv/strconv", "unicode/utf8/utf8", "time/time", "errors/errors":
			// We want to map these to their base package name so 'import "fmt"' works
			// Actually yaegi expects the full path as key in Use() if we want it to be isolatable?
			// No, i.Use(symbols) just merges symbols into the interpreter's symbol table.
			symbols[pkg] = export
		}
	}
}

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

// Execute takes raw bytes and a string of Go code (from AI) and runs it
func (e *Engine) Execute(rawData []byte, goCode string) (map[string]interface{}, error) {
	// Create a fresh interpreter for EACH execution to avoid package/import conflicts
	i := interp.New(interp.Options{})
	_ = i.Use(symbols) // Use restricted symbols!

	_, err := i.Eval(goCode)
	if err != nil {
		return nil, fmt.Errorf("COMPILE_ERROR: %v", err)
	}

	// Look for the "Parse" function in the dynamic package
	v, err := i.Eval("dynamic.Parse")
	if err != nil {
		return nil, fmt.Errorf("RECOVERY_ERROR: could not find Parse function: %v", err)
	}

	// Call the function
	fn, ok := v.Interface().(func([]byte) map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("RECOVERY_ERROR: Parse function has wrong signature")
	}

	// Safety wrapper for the call to catch panics if needed (simplified for now)
	result := fn(rawData)

	return result, nil
}
