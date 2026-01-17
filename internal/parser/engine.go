package parser

import (
	"fmt"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

type Engine struct {
	interpreter *interp.Interpreter
}

func NewEngine() *Engine {
	i := interp.New(interp.Options{})
	i.Use(stdlib.Symbols) // Allow AI to use standard Go libraries (math, encoding, etc.)
	return &Engine{interpreter: i}
}

// Execute takes raw bytes and a string of Go code (from AI) and runs it
func (e *Engine) Execute(rawData []byte, goCode string) (map[string]interface{}, error) {
	_, err := e.interpreter.Eval(goCode)
	if err != nil {
		return nil, fmt.Errorf("failed to eval AI code: %v", err)
	}

	// Look for the "Parse" function in the dynamic package
	v, err := e.interpreter.Eval("dynamic.Parse")
	if err != nil {
		return nil, fmt.Errorf("could not find Parse function: %v", err)
	}

	// Call the function
	fn := v.Interface().(func([]byte) map[string]interface{})
	result := fn(rawData)

	return result, nil
}
