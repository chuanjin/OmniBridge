package parser

import (
	"context"
	"fmt"
	"sync"
	"time"

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

type ParserFunc func([]byte) map[string]interface{}

type Engine struct {
	cache map[string]ParserFunc
	mu    sync.RWMutex
}

func NewEngine() *Engine {
	return &Engine{
		cache: make(map[string]ParserFunc),
	}
}

// Execute takes raw bytes and a string of Go code (from AI) and runs it.
// It uses a cache to avoid redundant compilation of the same code.
// It executes with a default timeout of 50ms to prevent infinite loops.
func (e *Engine) Execute(id string, rawData []byte, goCode string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	return e.ExecuteWithContext(ctx, id, rawData, goCode)
}

// ExecuteWithContext allows passing a custom context for execution.
func (e *Engine) ExecuteWithContext(ctx context.Context, id string, rawData []byte, goCode string) (map[string]interface{}, error) {
	// 1. Check if we already have a compiled version for this ID
	e.mu.RLock()
	fn, exists := e.cache[id]
	e.mu.RUnlock()

	if !exists {
		// 2. Compile and cache
		e.mu.Lock()
		// Double check after acquiring lock
		var err error
		if fn, exists = e.cache[id]; !exists {
			fn, err = e.compile(goCode)
			if err != nil {
				e.mu.Unlock()
				return nil, err
			}
			e.cache[id] = fn
		}
		e.mu.Unlock()
	}

	// 3. Execute with timeout protection
	type result struct {
		res map[string]interface{}
		err error
	}
	resChan := make(chan result, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				resChan <- result{err: fmt.Errorf("PANIC: %v", r)}
			}
		}()
		resChan <- result{res: fn(rawData)}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("EXECUTION_TIMEOUT: parser exceeded time limit")
	case r := <-resChan:
		return r.res, r.err
	}
}

func (e *Engine) compile(goCode string) (ParserFunc, error) {
	i := interp.New(interp.Options{})
	_ = i.Use(symbols)

	_, err := i.Eval(goCode)
	if err != nil {
		return nil, fmt.Errorf("COMPILE_ERROR: %v", err)
	}

	v, err := i.Eval("dynamic.Parse")
	if err != nil {
		return nil, fmt.Errorf("RECOVERY_ERROR: could not find Parse function: %v", err)
	}

	fn, ok := v.Interface().(func([]byte) map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("RECOVERY_ERROR: Parse function has wrong signature")
	}

	return fn, nil
}

// ClearCache removes cached parsers, useful if code changes
func (e *Engine) ClearCache(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.cache, id)
}

// CompileAndCache pre-compiles code for an ID
func (e *Engine) CompileAndCache(id string, goCode string) error {
	fn, err := e.compile(goCode)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cache[id] = fn
	return nil
}
