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

// CreateAM319Codec returns the Milesight AM319 codec script
func CreateAM319Codec() string {
	return `
// Milesight AM319 Environmental Sensor Codec
// Supports: Temperature, Humidity, PIR, Light, CO2, TVOC, Pressure, HCHO, PM2.5, PM10

function Encode(fPort, obj) {
    var bytes = [];

    // Helper function for random variations
    function randomVariation(base, variance) {
        return base + (Math.random() - 0.5) * 2 * variance;
    }

    // Sensor values with realistic random variations (based on real AM319 data)
    var temperature = obj.temperature !== undefined ? obj.temperature : randomVariation(19.2, 0.5);
    var humidity = obj.humidity !== undefined ? obj.humidity : randomVariation(31, 2);
    var pir = obj.pir !== undefined ? obj.pir : (Math.random() < 0.1 ? "trigger" : "idle");
    var light_level = obj.light_level !== undefined ? obj.light_level : Math.floor(randomVariation(1, 0.5));
    var co2 = obj.co2 !== undefined ? obj.co2 : Math.floor(randomVariation(465, 10));
    var tvoc = obj.tvoc !== undefined ? obj.tvoc : randomVariation(0.69, 0.05);
    var pressure = obj.pressure !== undefined ? obj.pressure : randomVariation(989.8, 0.5);
    var hcho = obj.hcho !== undefined ? obj.hcho : randomVariation(0.02, 0.005);
    var pm2_5 = obj.pm2_5 !== undefined ? obj.pm2_5 : Math.floor(randomVariation(12, 2));
    var pm10 = obj.pm10 !== undefined ? obj.pm10 : Math.floor(randomVariation(12, 2));

    // Temperature (Channel 0x03, Type 0x67)
    bytes.push(0x03);
    bytes.push(0x67);
    var tempInt = Math.round(temperature * 10);
    bytes.push(tempInt & 0xFF);
    bytes.push((tempInt >> 8) & 0xFF);

    // Humidity (Channel 0x04, Type 0x68)
    bytes.push(0x04);
    bytes.push(0x68);
    bytes.push(Math.round(humidity * 2));

    // PIR (Channel 0x05, Type 0x00)
    bytes.push(0x05);
    bytes.push(0x00);
    bytes.push(pir === "trigger" ? 1 : 0);

    // Light Level (Channel 0x06, Type 0xCB)
    bytes.push(0x06);
    bytes.push(0xCB);
    bytes.push(light_level & 0xFF);

    // CO2 (Channel 0x07, Type 0x7D)
    bytes.push(0x07);
    bytes.push(0x7D);
    var co2Int = Math.round(co2);
    bytes.push(co2Int & 0xFF);
    bytes.push((co2Int >> 8) & 0xFF);

    // TVOC (Channel 0x08, Type 0x7D) - IAQ format
    bytes.push(0x08);
    bytes.push(0x7D);
    var tvocInt = Math.round(tvoc * 100);
    bytes.push(tvocInt & 0xFF);
    bytes.push((tvocInt >> 8) & 0xFF);

    // Pressure (Channel 0x09, Type 0x73)
    bytes.push(0x09);
    bytes.push(0x73);
    var pressureInt = Math.round(pressure * 10);
    bytes.push(pressureInt & 0xFF);
    bytes.push((pressureInt >> 8) & 0xFF);

    // HCHO (Channel 0x0A, Type 0x7D)
    bytes.push(0x0A);
    bytes.push(0x7D);
    var hchoInt = Math.round(hcho * 100);
    bytes.push(hchoInt & 0xFF);
    bytes.push((hchoInt >> 8) & 0xFF);

    // PM2.5 (Channel 0x0B, Type 0x7D)
    bytes.push(0x0B);
    bytes.push(0x7D);
    var pm25Int = Math.round(pm2_5);
    bytes.push(pm25Int & 0xFF);
    bytes.push((pm25Int >> 8) & 0xFF);

    // PM10 (Channel 0x0C, Type 0x7D)
    bytes.push(0x0C);
    bytes.push(0x7D);
    var pm10Int = Math.round(pm10);
    bytes.push(pm10Int & 0xFF);
    bytes.push((pm10Int >> 8) & 0xFF);

    // Return with fPort 85 (AM319 standard port)
    return {
        fPort: 85,
        bytes: bytes
    };
}

function Decode(fPort, bytes) {
    var decoded = {};

    for (var i = 0; i < bytes.length; ) {
        var channel_id = bytes[i++];
        var channel_type = bytes[i++];

        // TEMPERATURE
        if (channel_id === 0x03 && channel_type === 0x67) {
            decoded.temperature = readInt16LE(bytes.slice(i, i + 2)) / 10;
            i += 2;
        }
        // HUMIDITY
        else if (channel_id === 0x04 && channel_type === 0x68) {
            decoded.humidity = bytes[i] / 2;
            i += 1;
        }
        // PIR
        else if (channel_id === 0x05 && channel_type === 0x00) {
            decoded.pir = bytes[i] === 1 ? "trigger" : "idle";
            i += 1;
        }
        // LIGHT
        else if (channel_id === 0x06 && channel_type === 0xcb) {
            decoded.light_level = bytes[i];
            i += 1;
        }
        // CO2
        else if (channel_id === 0x07 && channel_type === 0x7d) {
            decoded.co2 = readUInt16LE(bytes.slice(i, i + 2));
            i += 2;
        }
        // TVOC (iaq)
        else if (channel_id === 0x08 && channel_type === 0x7d) {
            decoded.tvoc = readUInt16LE(bytes.slice(i, i + 2)) / 100;
            i += 2;
        }
        // PRESSURE
        else if (channel_id === 0x09 && channel_type === 0x73) {
            decoded.pressure = readUInt16LE(bytes.slice(i, i + 2)) / 10;
            i += 2;
        }
        // HCHO
        else if (channel_id === 0x0a && channel_type === 0x7d) {
            decoded.hcho = readUInt16LE(bytes.slice(i, i + 2)) / 100;
            i += 2;
        }
        // PM2.5
        else if (channel_id === 0x0b && channel_type === 0x7d) {
            decoded.pm2_5 = readUInt16LE(bytes.slice(i, i + 2));
            i += 2;
        }
        // PM10
        else if (channel_id === 0x0c && channel_type === 0x7d) {
            decoded.pm10 = readUInt16LE(bytes.slice(i, i + 2));
            i += 2;
        }
        // O3
        else if (channel_id === 0x0d && channel_type === 0x7d) {
            decoded.o3 = readUInt16LE(bytes.slice(i, i + 2)) / 100;
            i += 2;
        }
        else {
            break;
        }
    }

    return decoded;
}

function readUInt16LE(bytes) {
    var value = (bytes[1] << 8) + bytes[0];
    return value & 0xffff;
}

function readInt16LE(bytes) {
    var ref = readUInt16LE(bytes);
    return ref > 0x7fff ? ref - 0x10000 : ref;
}
`
}

