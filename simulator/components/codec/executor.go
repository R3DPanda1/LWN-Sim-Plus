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
	// ErrOnUplinkNotFound is returned when OnUplink function is not defined
	ErrOnUplinkNotFound = errors.New("OnUplink function not found")
	// ErrInvalidReturnType is returned when the codec returns an invalid type
	ErrInvalidReturnType = errors.New("invalid return type from codec")
)

// Executor manages JavaScript codec execution with goja
type Executor struct {
	vmPool  *VMPool
	timeout time.Duration
	metrics *ExecutorMetrics
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

// ExecuteEncode executes the OnUplink function from a JavaScript codec
// Parameters:
//   - script: The JavaScript code containing the OnUplink function
//   - state: Device state for stateful encoding
//   - device: Device interface for accessing configuration (send interval, etc.)
//
// Returns the encoded byte array, the fPort (from device or codec), and any error
func (e *Executor) ExecuteEncode(script string, state *State, device DeviceInterface) ([]byte, uint8, error) {
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
		data, returnedFPort, err := e.executeEncodeInVM(vm, script, state, device)
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
		// We don't have a default fPort anymore, return 1 as fallback
		return nil, 1, ErrTimeout
	}
}

// executeEncodeInVM performs the actual encoding in the VM
func (e *Executor) executeEncodeInVM(vm *goja.Runtime, script string, state *State, device DeviceInterface) ([]byte, uint8, error) {
	// Inject conversion helpers (hexToBytes, base64ToBytes)
	if err := InjectConversionHelpers(vm); err != nil {
		return nil, 1, fmt.Errorf("failed to inject conversion helpers: %w", err)
	}

	// Inject state helper functions
	if err := InjectStateHelpers(vm, state); err != nil {
		return nil, 1, fmt.Errorf("failed to inject state helpers: %w", err)
	}

	// Inject device helpers (getSendInterval, setSendInterval)
	if device != nil {
		if err := InjectDeviceHelpers(vm, device); err != nil {
			return nil, 1, fmt.Errorf("failed to inject device helpers: %w", err)
		}
	}

	// Execute the script to define the OnUplink function
	_, err := vm.RunString(script)
	if err != nil {
		return nil, 1, fmt.Errorf("%w: script compilation error: %v", ErrInvalidScript, err)
	}

	// Get the OnUplink function
	onUplinkFunc, ok := goja.AssertFunction(vm.Get("OnUplink"))
	if !ok {
		return nil, 1, ErrOnUplinkNotFound
	}

	// Call OnUplink() with no arguments
	result, err := onUplinkFunc(goja.Undefined())
	if err != nil {
		return nil, 1, fmt.Errorf("OnUplink execution error (check JavaScript): %w", err)
	}

	// Convert result to byte array and extract fPort if provided
	// Default fPort is 1 if not specified by codec
	bytes, returnedFPort, err := e.convertToBytesWithFPort(vm, result, 1)
	if err != nil {
		return nil, 1, err
	}

	return bytes, returnedFPort, nil
}

// ExecuteDecode executes the OnDownlink function from a JavaScript codec
// Parameters:
//   - script: The JavaScript code containing the OnDownlink function
//   - bytes: The byte array to decode
//   - fPort: The LoRaWAN fPort
//   - state: Device state for stateful decoding
//   - device: Device interface for accessing configuration
//
// OnDownlink is executed for its side effects (log, setState, setSendInterval).
// Any return value from the JavaScript function is ignored.
func (e *Executor) ExecuteDecode(script string, bytes []byte, fPort uint8, state *State, device DeviceInterface) error {
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
	errChan := make(chan error, 1)

	// Execute in goroutine
	go func() {
		err := e.executeDecodeInVM(vm, script, bytes, fPort, state, device)
		errChan <- err
	}()

	// Wait for result or timeout
	select {
	case err := <-errChan:
		if err != nil && e.metrics != nil {
			e.metrics.mu.Lock()
			e.metrics.TotalErrors++
			e.metrics.mu.Unlock()
		}
		return err
	case <-ctx.Done():
		if e.metrics != nil {
			e.metrics.mu.Lock()
			e.metrics.TotalTimeouts++
			e.metrics.mu.Unlock()
		}
		return ErrTimeout
	}
}

// executeDecodeInVM performs the actual decoding in the VM
func (e *Executor) executeDecodeInVM(vm *goja.Runtime, script string, bytes []byte, fPort uint8, state *State, device DeviceInterface) error {
	// Inject conversion helpers (hexToBytes, base64ToBytes)
	if err := InjectConversionHelpers(vm); err != nil {
		return fmt.Errorf("failed to inject conversion helpers: %w", err)
	}

	// Inject state helper functions
	if err := InjectStateHelpers(vm, state); err != nil {
		return fmt.Errorf("failed to inject state helpers: %w", err)
	}

	// Inject device helpers (getSendInterval, setSendInterval, log)
	if device != nil {
		if err := InjectDeviceHelpers(vm, device); err != nil {
			return fmt.Errorf("failed to inject device helpers: %w", err)
		}
	}

	// Execute the script to define the OnDownlink function
	_, err := vm.RunString(script)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidScript, err)
	}

	// Get the OnDownlink function (optional)
	onDownlinkFunc, ok := goja.AssertFunction(vm.Get("OnDownlink"))
	if !ok {
		// OnDownlink is optional, nothing to do
		return nil
	}

	// Convert bytes to JS array
	jsBytes := make([]interface{}, len(bytes))
	for i, b := range bytes {
		jsBytes[i] = b
	}

	// Call OnDownlink(bytes, fPort) - executed for side effects only
	_, err = onDownlinkFunc(goja.Undefined(), vm.ToValue(jsBytes), vm.ToValue(fPort))
	if err != nil {
		return fmt.Errorf("OnDownlink execution error: %w", err)
	}

	return nil
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
