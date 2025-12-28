package codec

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dop251/goja"
)

var (
	// ErrTimeout is returned when codec execution exceeds the timeout
	ErrTimeout = errors.New("codec execution timeout")
	// ErrInvalidScript is returned when the JavaScript code is invalid
	ErrInvalidScript = errors.New("invalid JavaScript code")
	// ErrEncodeFunctionNotFound is returned when Encode function is not defined
	ErrEncodeFunctionNotFound = errors.New("Encode function not found")
	// ErrDecodeFunctionNotFound is returned when Decode function is not defined
	ErrDecodeFunctionNotFound = errors.New("Decode function not found")
	// ErrInvalidReturnType is returned when the codec returns an invalid type
	ErrInvalidReturnType = errors.New("invalid return type from codec")
)

// Executor manages JavaScript codec execution with goja
type Executor struct {
	vmPool    *VMPool
	timeout   time.Duration
	mu        sync.RWMutex
	metrics   *ExecutorMetrics
}

// ExecutorMetrics tracks codec execution statistics
type ExecutorMetrics struct {
	TotalExecutions uint64
	TotalErrors     uint64
	TotalTimeouts   uint64
	mu              sync.RWMutex
}

// ExecutorConfig holds configuration for the Executor
type ExecutorConfig struct {
	MaxVMs        int
	Timeout       time.Duration
	EnableMetrics bool
}

// DefaultExecutorConfig returns default configuration
func DefaultExecutorConfig() *ExecutorConfig {
	return &ExecutorConfig{
		MaxVMs:        100,
		Timeout:       100 * time.Millisecond,
		EnableMetrics: true,
	}
}

// NewExecutor creates a new codec executor
func NewExecutor(config *ExecutorConfig) *Executor {
	if config == nil {
		config = DefaultExecutorConfig()
	}

	return &Executor{
		vmPool:  NewVMPool(config.MaxVMs),
		timeout: config.Timeout,
		metrics: &ExecutorMetrics{},
	}
}

// ExecuteEncode executes the Encode function from a JavaScript codec
// Parameters:
//   - script: The JavaScript code containing the Encode function
//   - fPort: The LoRaWAN fPort (default value, can be overridden by JS)
//   - obj: The input object to encode (as a map)
//   - state: Device state for stateful encoding (optional, can be nil)
//
// Returns the encoded byte array, the fPort (from JS or default), and any error
func (e *Executor) ExecuteEncode(script string, fPort uint8, obj map[string]interface{}, state *State) ([]byte, uint8, error) {
	// Record metrics
	if e.metrics != nil {
		e.metrics.mu.Lock()
		e.metrics.TotalExecutions++
		e.metrics.mu.Unlock()
	}

	// Get a VM from the pool
	vm := e.vmPool.Get()
	defer e.vmPool.Put(vm)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	// Channel to receive result
	type result struct {
		data  []byte
		fPort uint8
		err   error
	}
	resultChan := make(chan result, 1)

	// Execute in goroutine to support timeout
	go func() {
		data, returnedFPort, err := e.executeEncodeInVM(vm, script, fPort, obj, state)
		resultChan <- result{data: data, fPort: returnedFPort, err: err}
	}()

	// Wait for result or timeout
	select {
	case res := <-resultChan:
		if res.err != nil && e.metrics != nil {
			e.metrics.mu.Lock()
			e.metrics.TotalErrors++
			e.metrics.mu.Unlock()
		}
		return res.data, res.fPort, res.err
	case <-ctx.Done():
		if e.metrics != nil {
			e.metrics.mu.Lock()
			e.metrics.TotalTimeouts++
			e.metrics.mu.Unlock()
		}
		return nil, fPort, ErrTimeout
	}
}

// executeEncodeInVM performs the actual encoding in the VM
func (e *Executor) executeEncodeInVM(vm *goja.Runtime, script string, fPort uint8, obj map[string]interface{}, state *State) ([]byte, uint8, error) {
	// Inject state helper functions if state is provided
	if state != nil {
		if err := InjectStateHelpers(vm, state); err != nil {
			return nil, fPort, fmt.Errorf("failed to inject state helpers: %w", err)
		}
	}

	// Execute the script to define the Encode function
	_, err := vm.RunString(script)
	if err != nil {
		return nil, fPort, fmt.Errorf("%w: script compilation error: %v", ErrInvalidScript, err)
	}

	// Get the Encode function
	encodeFunc, ok := goja.AssertFunction(vm.Get("Encode"))
	if !ok {
		return nil, fPort, ErrEncodeFunctionNotFound
	}

	// Call Encode(fPort, obj)
	result, err := encodeFunc(goja.Undefined(), vm.ToValue(fPort), vm.ToValue(obj))
	if err != nil {
		return nil, fPort, fmt.Errorf("encode execution error (check JavaScript): %w", err)
	}

	// Convert result to byte array and extract fPort if provided
	bytes, returnedFPort, err := e.convertToBytesWithFPort(vm, result, fPort)
	if err != nil {
		return nil, fPort, err
	}

	return bytes, returnedFPort, nil
}

// ExecuteDecode executes the Decode function from a JavaScript codec
// Parameters:
//   - script: The JavaScript code containing the Decode function
//   - fPort: The LoRaWAN fPort
//   - bytes: The byte array to decode
//   - state: Device state for stateful decoding (optional, can be nil)
//
// Returns the decoded object as a map and any error
func (e *Executor) ExecuteDecode(script string, fPort uint8, bytes []byte, state *State) (map[string]interface{}, error) {
	// Record metrics
	if e.metrics != nil {
		e.metrics.mu.Lock()
		e.metrics.TotalExecutions++
		e.metrics.mu.Unlock()
	}

	// Get a VM from the pool
	vm := e.vmPool.Get()
	defer e.vmPool.Put(vm)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	// Channel to receive result
	type result struct {
		data map[string]interface{}
		err  error
	}
	resultChan := make(chan result, 1)

	// Execute in goroutine
	go func() {
		data, err := e.executeDecodeInVM(vm, script, fPort, bytes, state)
		resultChan <- result{data: data, err: err}
	}()

	// Wait for result or timeout
	select {
	case res := <-resultChan:
		if res.err != nil && e.metrics != nil {
			e.metrics.mu.Lock()
			e.metrics.TotalErrors++
			e.metrics.mu.Unlock()
		}
		return res.data, res.err
	case <-ctx.Done():
		if e.metrics != nil {
			e.metrics.mu.Lock()
			e.metrics.TotalTimeouts++
			e.metrics.mu.Unlock()
		}
		return nil, ErrTimeout
	}
}

// executeDecodeInVM performs the actual decoding in the VM
func (e *Executor) executeDecodeInVM(vm *goja.Runtime, script string, fPort uint8, bytes []byte, state *State) (map[string]interface{}, error) {
	// Inject state helper functions if state is provided
	if state != nil {
		if err := InjectStateHelpers(vm, state); err != nil {
			return nil, fmt.Errorf("failed to inject state helpers: %w", err)
		}
	}

	// Execute the script to define the Decode function
	_, err := vm.RunString(script)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidScript, err)
	}

	// Get the Decode function
	decodeFunc, ok := goja.AssertFunction(vm.Get("Decode"))
	if !ok {
		return nil, ErrDecodeFunctionNotFound
	}

	// Convert bytes to JS array
	jsBytes := make([]interface{}, len(bytes))
	for i, b := range bytes {
		jsBytes[i] = b
	}

	// Call Decode(fPort, bytes)
	result, err := decodeFunc(goja.Undefined(), vm.ToValue(fPort), vm.ToValue(jsBytes))
	if err != nil {
		return nil, fmt.Errorf("decode execution error: %w", err)
	}

	// Convert result to map
	resultMap := result.Export()
	if resultMap == nil {
		return make(map[string]interface{}), nil
	}

	objMap, ok := resultMap.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: expected object, got %T", ErrInvalidReturnType, resultMap)
	}

	return objMap, nil
}

// convertToBytesWithFPort converts a goja.Value to a byte slice and extracts fPort if present
// Supports two formats:
//   1. Legacy: [byte1, byte2, ...] - returns bytes with default fPort
//   2. New: {fPort: 3, bytes: [byte1, byte2, ...]} - returns bytes with extracted fPort
func (e *Executor) convertToBytesWithFPort(vm *goja.Runtime, value goja.Value, defaultFPort uint8) ([]byte, uint8, error) {
	exported := value.Export()
	if exported == nil {
		return []byte{}, defaultFPort, nil
	}

	// Check if it's an object with fPort and bytes fields (new format)
	if obj, ok := exported.(map[string]interface{}); ok {
		// Extract fPort if present
		fPort := defaultFPort
		if fPortVal, hasFPort := obj["fPort"]; hasFPort {
			switch fp := fPortVal.(type) {
			case int64:
				if fp < 0 || fp > 255 {
					return nil, defaultFPort, fmt.Errorf("%w: fPort value out of range: %d", ErrInvalidReturnType, fp)
				}
				fPort = uint8(fp)
			case float64:
				if fp < 0 || fp > 255 {
					return nil, defaultFPort, fmt.Errorf("%w: fPort value out of range: %f", ErrInvalidReturnType, fp)
				}
				fPort = uint8(fp)
			case int:
				if fp < 0 || fp > 255 {
					return nil, defaultFPort, fmt.Errorf("%w: fPort value out of range: %d", ErrInvalidReturnType, fp)
				}
				fPort = uint8(fp)
			default:
				return nil, defaultFPort, fmt.Errorf("%w: invalid fPort type: %T", ErrInvalidReturnType, fPortVal)
			}
		}

		// Extract bytes array
		if bytesVal, hasBytes := obj["bytes"]; hasBytes {
			if arr, ok := bytesVal.([]interface{}); ok {
				bytes, err := e.arrayToBytes(arr)
				if err != nil {
					return nil, defaultFPort, err
				}
				return bytes, fPort, nil
			}
			return nil, defaultFPort, fmt.Errorf("%w: bytes field must be an array", ErrInvalidReturnType)
		}

		return nil, defaultFPort, fmt.Errorf("%w: object must have 'bytes' field", ErrInvalidReturnType)
	}

	// Legacy format: plain array
	if arr, ok := exported.([]interface{}); ok {
		bytes, err := e.arrayToBytes(arr)
		if err != nil {
			return nil, defaultFPort, err
		}
		return bytes, defaultFPort, nil
	}

	return nil, defaultFPort, fmt.Errorf("%w: expected array or object with {fPort, bytes}, got %T", ErrInvalidReturnType, exported)
}

// arrayToBytes converts an array of interfaces to bytes
func (e *Executor) arrayToBytes(arr []interface{}) ([]byte, error) {
	bytes := make([]byte, len(arr))
	for i, v := range arr {
		switch num := v.(type) {
		case int64:
			if num < 0 || num > 255 {
				return nil, fmt.Errorf("%w: byte value out of range: %d", ErrInvalidReturnType, num)
			}
			bytes[i] = byte(num)
		case float64:
			if num < 0 || num > 255 {
				return nil, fmt.Errorf("%w: byte value out of range: %f", ErrInvalidReturnType, num)
			}
			bytes[i] = byte(num)
		case int:
			if num < 0 || num > 255 {
				return nil, fmt.Errorf("%w: byte value out of range: %d", ErrInvalidReturnType, num)
			}
			bytes[i] = byte(num)
		default:
			return nil, fmt.Errorf("%w: invalid array element type: %T", ErrInvalidReturnType, v)
		}
	}
	return bytes, nil
}

// convertToBytes converts a goja.Value to a byte slice (legacy method for backward compatibility)
func (e *Executor) convertToBytes(vm *goja.Runtime, value goja.Value) ([]byte, error) {
	bytes, _, err := e.convertToBytesWithFPort(vm, value, 1)
	return bytes, err
}

// GetMetrics returns current executor metrics
func (e *Executor) GetMetrics() ExecutorMetrics {
	e.metrics.mu.RLock()
	defer e.metrics.mu.RUnlock()
	return *e.metrics
}

// ResetMetrics resets all metrics to zero
func (e *Executor) ResetMetrics() {
	e.metrics.mu.Lock()
	defer e.metrics.mu.Unlock()
	e.metrics.TotalExecutions = 0
	e.metrics.TotalErrors = 0
	e.metrics.TotalTimeouts = 0
}

// Close closes the executor and releases resources
func (e *Executor) Close() {
	if e.vmPool != nil {
		e.vmPool.Close()
	}
}
