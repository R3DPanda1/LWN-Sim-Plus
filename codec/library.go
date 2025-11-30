package codec

import (
	"fmt"

	"github.com/dop251/goja"
)

// InjectStateHelpers injects state management helper functions into the JavaScript VM
// These functions allow JavaScript codecs to access and modify device state
func InjectStateHelpers(vm *goja.Runtime, state *State) error {
	if vm == nil {
		return fmt.Errorf("VM cannot be nil")
	}
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	}

	// getCounter(name) - Get a counter value
	vm.Set("getCounter", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("getCounter requires a name argument"))
		}

		name := call.Argument(0).String()
		value := state.GetCounter(name)
		return vm.ToValue(value)
	})

	// setCounter(name, value) - Set a counter value
	vm.Set("setCounter", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("setCounter requires name and value arguments"))
		}

		name := call.Argument(0).String()
		value := call.Argument(1).ToInteger()
		state.SetCounter(name, value)
		return goja.Undefined()
	})

	// getState(name) - Get a state variable
	vm.Set("getState", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("getState requires a name argument"))
		}

		name := call.Argument(0).String()
		value := state.GetVariable(name)
		if value == nil {
			return goja.Null()
		}
		return vm.ToValue(value)
	})

	// setState(name, value) - Set a state variable
	vm.Set("setState", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("setState requires name and value arguments"))
		}

		name := call.Argument(0).String()
		value := call.Argument(1).Export()
		state.SetVariable(name, value)
		return goja.Undefined()
	})

	// getPreviousPayload() - Get the last payload sent
	vm.Set("getPreviousPayload", func(call goja.FunctionCall) goja.Value {
		payload := state.GetPreviousPayload()
		if payload == nil {
			return goja.Null()
		}

		// Convert []byte to JavaScript array
		jsArray := make([]interface{}, len(payload))
		for i, b := range payload {
			jsArray[i] = int(b)
		}
		return vm.ToValue(jsArray)
	})

	// getPreviousPayloads(n) - Get the last n payloads
	vm.Set("getPreviousPayloads", func(call goja.FunctionCall) goja.Value {
		n := 1
		if len(call.Arguments) > 0 {
			n = int(call.Argument(0).ToInteger())
		}

		payloads := state.GetPreviousPayloads(n)
		if len(payloads) == 0 {
			return vm.ToValue([]interface{}{})
		}

		// Convert [][]byte to JavaScript array of arrays
		jsArrays := make([]interface{}, len(payloads))
		for i, payload := range payloads {
			jsArray := make([]interface{}, len(payload))
			for j, b := range payload {
				jsArray[j] = int(b)
			}
			jsArrays[i] = jsArray
		}
		return vm.ToValue(jsArrays)
	})

	// log(message) - Debug logging (console.log alternative)
	vm.Set("log", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			message := call.Argument(0).String()
			// In the future, we can integrate this with the device logger
			// For now, we'll just create a marker for future implementation
			_ = message
			// fmt.Printf("[Codec Log]: %s\n", message)
		}
		return goja.Undefined()
	})

	return nil
}

// InjectMathHelpers injects additional math utilities for codecs
// This is optional but can be useful for complex payload generation
func InjectMathHelpers(vm *goja.Runtime) error {
	if vm == nil {
		return fmt.Errorf("VM cannot be nil")
	}

	// random(min, max) - Generate random number (useful for simulation)
	vm.Set("random", func(call goja.FunctionCall) goja.Value {
		min := 0.0
		max := 1.0

		if len(call.Arguments) >= 1 {
			min = call.Argument(0).ToFloat()
		}
		if len(call.Arguments) >= 2 {
			max = call.Argument(1).ToFloat()
		}

		// Simple random implementation
		// In production, you'd use crypto/rand for better randomness
		value := min + (max-min)*0.5 // Placeholder
		return vm.ToValue(value)
	})

	// clamp(value, min, max) - Clamp value between min and max
	vm.Set("clamp", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 3 {
			panic(vm.NewTypeError("clamp requires value, min, and max arguments"))
		}

		value := call.Argument(0).ToFloat()
		min := call.Argument(1).ToFloat()
		max := call.Argument(2).ToFloat()

		if value < min {
			value = min
		} else if value > max {
			value = max
		}

		return vm.ToValue(value)
	})

	return nil
}

// CreateSampleCodec returns a sample codec script for testing
func CreateSampleCodec() string {
	return `
function Encode(fPort, obj) {
    var counter = getCounter("messageCount");
    setCounter("messageCount", counter + 1);

    var bytes = [];

    // Add counter (2 bytes)
    bytes.push((counter >> 8) & 0xFF);
    bytes.push(counter & 0xFF);

    // Add temperature if provided
    if (obj.temperature !== undefined) {
        var temp = Math.round((obj.temperature + 50) * 2);
        bytes.push(temp & 0xFF);
    }

    // Add humidity if provided
    if (obj.humidity !== undefined) {
        bytes.push(obj.humidity & 0xFF);
    }

    return bytes;
}

function Decode(fPort, bytes) {
    var obj = {};

    if (bytes.length >= 2) {
        obj.counter = (bytes[0] << 8) | bytes[1];
    }

    if (bytes.length >= 3) {
        obj.temperature = (bytes[2] / 2) - 50;
    }

    if (bytes.length >= 4) {
        obj.humidity = bytes[3];
    }

    return obj;
}
`
}

