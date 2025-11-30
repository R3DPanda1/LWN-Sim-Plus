package codec

import (
	"testing"
	"time"
)

// TestExecutorSimpleEncode tests basic encoding functionality
func TestExecutorSimpleEncode(t *testing.T) {
	executor := NewExecutor(nil)
	defer executor.Close()

	script := `
function Encode(fPort, obj) {
    var bytes = [];
    bytes.push(obj.value);
    return bytes;
}
`

	obj := map[string]interface{}{
		"value": 42,
	}

	bytes, err := executor.ExecuteEncode(script, 1, obj, nil)
	if err != nil {
		t.Fatalf("ExecuteEncode failed: %v", err)
	}

	if len(bytes) != 1 {
		t.Fatalf("Expected 1 byte, got %d", len(bytes))
	}

	if bytes[0] != 42 {
		t.Fatalf("Expected byte value 42, got %d", bytes[0])
	}
}

// TestExecutorSimpleDecode tests basic decoding functionality
func TestExecutorSimpleDecode(t *testing.T) {
	executor := NewExecutor(nil)
	defer executor.Close()

	script := `
function Decode(fPort, bytes) {
    return {value: bytes[0]};
}
`

	bytes := []byte{42}

	obj, err := executor.ExecuteDecode(script, 1, bytes, nil)
	if err != nil {
		t.Fatalf("ExecuteDecode failed: %v", err)
	}

	value, ok := obj["value"]
	if !ok {
		t.Fatal("Expected 'value' field in decoded object")
	}

	// JavaScript numbers can be float64 or int64
	var valueInt int
	switch v := value.(type) {
	case float64:
		valueInt = int(v)
	case int64:
		valueInt = int(v)
	case int:
		valueInt = v
	default:
		t.Fatalf("Expected numeric type, got %T", value)
	}

	if valueInt != 42 {
		t.Fatalf("Expected value 42, got %v", value)
	}
}

// TestExecutorStatefulEncode tests encoding with state
func TestExecutorStatefulEncode(t *testing.T) {
	executor := NewExecutor(nil)
	defer executor.Close()

	script := `
function Encode(fPort, obj) {
    var counter = getCounter("test");
    setCounter("test", counter + 1);
    var bytes = [counter >> 8, counter & 0xFF];
    return bytes;
}
`

	state := NewState("test-device")
	obj := map[string]interface{}{}

	// First execution
	bytes1, err := executor.ExecuteEncode(script, 1, obj, state)
	if err != nil {
		t.Fatalf("ExecuteEncode failed: %v", err)
	}

	if len(bytes1) != 2 {
		t.Fatalf("Expected 2 bytes, got %d", len(bytes1))
	}

	// Counter should be 0 initially
	if bytes1[0] != 0 || bytes1[1] != 0 {
		t.Fatalf("Expected counter 0, got %d", (int(bytes1[0])<<8)|int(bytes1[1]))
	}

	// Second execution
	bytes2, err := executor.ExecuteEncode(script, 1, obj, state)
	if err != nil {
		t.Fatalf("ExecuteEncode failed: %v", err)
	}

	// Counter should be 1 now
	counter := (int(bytes2[0]) << 8) | int(bytes2[1])
	if counter != 1 {
		t.Fatalf("Expected counter 1, got %d", counter)
	}
}

// TestExecutorMessageHistory tests message history functionality
func TestExecutorMessageHistory(t *testing.T) {
	executor := NewExecutor(nil)
	defer executor.Close()

	script := `
function Encode(fPort, obj) {
    var prev = getPreviousPayload();
    if (prev === null) {
        return [1, 2, 3];
    }
    return [prev[0] + 1, prev[1] + 1, prev[2] + 1];
}
`

	state := NewState("test-device")
	obj := map[string]interface{}{}

	// First execution - no history
	bytes1, err := executor.ExecuteEncode(script, 1, obj, state)
	if err != nil {
		t.Fatalf("ExecuteEncode failed: %v", err)
	}

	if len(bytes1) != 3 || bytes1[0] != 1 || bytes1[1] != 2 || bytes1[2] != 3 {
		t.Fatalf("Expected [1,2,3], got %v", bytes1)
	}

	// Add to history
	state.AddMessage(MessageRecord{
		FCnt:      1,
		Timestamp: time.Now(),
		Payload:   bytes1,
		FPort:     1,
	})

	// Second execution - should increment previous values
	bytes2, err := executor.ExecuteEncode(script, 1, obj, state)
	if err != nil {
		t.Fatalf("ExecuteEncode failed: %v", err)
	}

	if len(bytes2) != 3 || bytes2[0] != 2 || bytes2[1] != 3 || bytes2[2] != 4 {
		t.Fatalf("Expected [2,3,4], got %v", bytes2)
	}
}

// TestExecutorInvalidScript tests error handling for invalid JavaScript
func TestExecutorInvalidScript(t *testing.T) {
	executor := NewExecutor(nil)
	defer executor.Close()

	script := `
this is not valid javascript {{{
`

	obj := map[string]interface{}{"value": 42}

	_, err := executor.ExecuteEncode(script, 1, obj, nil)
	if err == nil {
		t.Fatal("Expected error for invalid script")
	}
}

// TestExecutorMissingEncodeFunction tests error when Encode function is missing
func TestExecutorMissingEncodeFunction(t *testing.T) {
	executor := NewExecutor(nil)
	defer executor.Close()

	script := `
function SomeOtherFunction() {
    return [1, 2, 3];
}
`

	obj := map[string]interface{}{"value": 42}

	_, err := executor.ExecuteEncode(script, 1, obj, nil)
	if err != ErrEncodeFunctionNotFound {
		t.Fatalf("Expected ErrEncodeFunctionNotFound, got %v", err)
	}
}

// TestExecutorTimeout tests execution timeout
func TestExecutorTimeout(t *testing.T) {
	config := &ExecutorConfig{
		MaxVMs:        10,
		Timeout:       10 * time.Millisecond, // Very short timeout
		EnableMetrics: true,
	}
	executor := NewExecutor(config)
	defer executor.Close()

	// Script with infinite loop
	script := `
function Encode(fPort, obj) {
    while(true) {
        // Infinite loop
    }
    return [1];
}
`

	obj := map[string]interface{}{"value": 42}

	_, err := executor.ExecuteEncode(script, 1, obj, nil)
	if err != ErrTimeout {
		t.Fatalf("Expected ErrTimeout, got %v", err)
	}

	// Check metrics
	metrics := executor.GetMetrics()
	if metrics.TotalTimeouts != 1 {
		t.Fatalf("Expected 1 timeout, got %d", metrics.TotalTimeouts)
	}
}

// TestExecutorMetrics tests metrics tracking
func TestExecutorMetrics(t *testing.T) {
	executor := NewExecutor(nil)
	defer executor.Close()

	script := CreateSampleCodec()
	obj := map[string]interface{}{
		"temperature": 22.5,
		"humidity":    65,
	}
	state := NewState("test-device")

	// Execute multiple times
	for i := 0; i < 5; i++ {
		_, err := executor.ExecuteEncode(script, 1, obj, state)
		if err != nil {
			t.Fatalf("ExecuteEncode failed: %v", err)
		}
	}

	metrics := executor.GetMetrics()
	if metrics.TotalExecutions != 5 {
		t.Fatalf("Expected 5 executions, got %d", metrics.TotalExecutions)
	}

	if metrics.TotalErrors != 0 {
		t.Fatalf("Expected 0 errors, got %d", metrics.TotalErrors)
	}
}

// TestExecutorRoundTrip tests encode and decode round-trip
func TestExecutorRoundTrip(t *testing.T) {
	executor := NewExecutor(nil)
	defer executor.Close()

	script := CreateSampleCodec()

	originalObj := map[string]interface{}{
		"temperature": 22.5,
		"humidity":    65,
	}

	state := NewState("test-device")

	// Encode
	bytes, err := executor.ExecuteEncode(script, 1, originalObj, state)
	if err != nil {
		t.Fatalf("ExecuteEncode failed: %v", err)
	}

	// Decode
	decodedObj, err := executor.ExecuteDecode(script, 1, bytes, state)
	if err != nil {
		t.Fatalf("ExecuteDecode failed: %v", err)
	}

	// Check temperature
	tempVal, ok := decodedObj["temperature"]
	if !ok {
		t.Fatal("temperature not found")
	}
	var temp float64
	switch v := tempVal.(type) {
	case float64:
		temp = v
	case int64:
		temp = float64(v)
	case int:
		temp = float64(v)
	default:
		t.Fatalf("temperature wrong type: %T", tempVal)
	}

	// Allow small floating point error
	if temp < 22.0 || temp > 23.0 {
		t.Fatalf("Expected temperature ~22.5, got %v", temp)
	}

	// Check humidity
	humVal, ok := decodedObj["humidity"]
	if !ok {
		t.Fatal("humidity not found")
	}
	var hum int
	switch v := humVal.(type) {
	case float64:
		hum = int(v)
	case int64:
		hum = int(v)
	case int:
		hum = v
	default:
		t.Fatalf("humidity wrong type: %T", humVal)
	}

	if hum != 65 {
		t.Fatalf("Expected humidity 65, got %v", hum)
	}
}


// BenchmarkExecutorEncode benchmarks codec encoding
func BenchmarkExecutorEncode(b *testing.B) {
	executor := NewExecutor(nil)
	defer executor.Close()

	script := CreateSampleCodec()
	obj := map[string]interface{}{
		"temperature": 22.5,
		"humidity":    65,
	}
	state := NewState("test-device")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.ExecuteEncode(script, 1, obj, state)
		if err != nil {
			b.Fatalf("ExecuteEncode failed: %v", err)
		}
	}
}

// BenchmarkExecutorDecode benchmarks codec decoding
func BenchmarkExecutorDecode(b *testing.B) {
	executor := NewExecutor(nil)
	defer executor.Close()

	script := CreateSampleCodec()
	bytes := []byte{0, 1, 145, 65}
	state := NewState("test-device")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.ExecuteDecode(script, 1, bytes, state)
		if err != nil {
			b.Fatalf("ExecuteDecode failed: %v", err)
		}
	}
}
