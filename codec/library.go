package codec

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/dop251/goja"
)

// InjectStateHelpers injects state management helper functions into the JavaScript VM
// Simplified to only include getState and setState (all-purpose state management)
func InjectStateHelpers(vm *goja.Runtime, state *State) error {
	if vm == nil {
		return fmt.Errorf("VM cannot be nil")
	}
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	}

	// getState(name) - Get a state variable (any type)
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

	// setState(name, value) - Set a state variable (any type)
	vm.Set("setState", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("setState requires name and value arguments"))
		}

		name := call.Argument(0).String()
		value := call.Argument(1).Export()
		state.SetVariable(name, value)
		return goja.Undefined()
	})

	return nil
}

// InjectConversionHelpers injects payload conversion helper functions into the JavaScript VM
// These allow explicit conversion from hex/base64 strings to byte arrays
func InjectConversionHelpers(vm *goja.Runtime) error {
	if vm == nil {
		return fmt.Errorf("VM cannot be nil")
	}

	// hexToBytes(hexString) - Convert hex string to byte array
	vm.Set("hexToBytes", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("hexToBytes requires hex string argument"))
		}

		hexStr := call.Argument(0).String()
		bytes, err := hex.DecodeString(hexStr)
		if err != nil {
			panic(vm.NewTypeError("Invalid hex string: " + err.Error()))
		}

		// Convert to JavaScript array
		arr := vm.NewArray()
		for i, b := range bytes {
			arr.Set(strconv.Itoa(i), vm.ToValue(int(b)))
		}
		return arr
	})

	// base64ToBytes(base64String) - Convert base64 string to byte array
	vm.Set("base64ToBytes", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("base64ToBytes requires base64 string argument"))
		}

		b64Str := call.Argument(0).String()
		bytes, err := base64.StdEncoding.DecodeString(b64Str)
		if err != nil {
			panic(vm.NewTypeError("Invalid base64 string: " + err.Error()))
		}

		// Convert to JavaScript array
		arr := vm.NewArray()
		for i, b := range bytes {
			arr.Set(strconv.Itoa(i), vm.ToValue(int(b)))
		}
		return arr
	})

	return nil
}

// DeviceInterface defines the interface for accessing device configuration from JavaScript
type DeviceInterface interface {
	GetSendInterval() time.Duration
	SetSendInterval(time.Duration)
	Print(content string, err error, printType int)
}

// InjectDeviceHelpers injects device configuration helper functions into the JavaScript VM
// These allow JavaScript codecs to read and modify device settings
func InjectDeviceHelpers(vm *goja.Runtime, device DeviceInterface) error {
	if vm == nil {
		return fmt.Errorf("VM cannot be nil")
	}
	if device == nil {
		return fmt.Errorf("device cannot be nil")
	}

	// getSendInterval() - Returns send interval in seconds
	vm.Set("getSendInterval", func() goja.Value {
		seconds := int(device.GetSendInterval().Seconds())
		return vm.ToValue(seconds)
	})

	// setSendInterval(seconds) - Changes send interval
	vm.Set("setSendInterval", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("setSendInterval requires seconds argument"))
		}

		seconds := call.Argument(0).ToInteger()
		device.SetSendInterval(time.Duration(seconds) * time.Second)
		return goja.Undefined()
	})

	// log(message) - Logs message to device console
	// Note: PrintBoth = 2 in util/const.go (iota starts after MAXFCNTGAP)
	vm.Set("log", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}

		message := call.Argument(0).String()
		device.Print("[CODEC] "+message, nil, 2) // printType 2 = PrintBoth
		return goja.Undefined()
	})

	return nil
}

// CreateAM319Codec returns the Milesight AM319 codec script
func CreateAM319Codec() string {
	return `
// Milesight AM319 Indoor Ambiance Monitoring Sensor Codec

function initState() {
    if (getState('battery') === null) setState('battery', 100);
    if (getState('tvocMode') === null) setState('tvocMode', 0);
}

function uint16LE(value) {
    return [value & 0xFF, (value >> 8) & 0xFF];
}

function int16LE(value) {
    if (value < 0) value = 0x10000 + value;
    return [value & 0xFF, (value >> 8) & 0xFF];
}

function decodeUint16LE(bytes, offset) {
    return bytes[offset] | (bytes[offset + 1] << 8);
}

function generateSensorValues() {
    var values = {};
    var baseTemp = getState('baseTemperature') || 22.5;
    var baseHumidity = getState('baseHumidity') || 45;
    var baseCO2 = getState('baseCO2') || 550;
    var tvocMode = getState('tvocMode') || 0;

    values.temperature = baseTemp + (Math.random() - 0.5) * 2;
    values.humidity = baseHumidity + (Math.random() - 0.5) * 10;
    values.pir = Math.random() > 0.7 ? 1 : 0;
    values.lightLevel = Math.floor(Math.random() * 6);
    values.co2 = Math.max(400, Math.floor(baseCO2 + (Math.random() - 0.5) * 100));
    values.pressure = 1013.25 + (Math.random() - 0.5) * 10;
    values.hcho = Math.max(0.01, 0.03 + (Math.random() - 0.5) * 0.02);
    values.pm25 = Math.max(0, Math.floor(25 + (Math.random() - 0.5) * 20));
    values.pm10 = Math.max(0, Math.floor(35 + (Math.random() - 0.5) * 30));

    if (tvocMode === 0) {
        values.tvocLevel = 1.5 + (Math.random() - 0.5) * 0.5;
    } else {
        values.tvocConcentration = Math.floor(200 + (Math.random() - 0.5) * 100);
    }

    return values;
}

function OnUplink() {
    initState();
    var values = generateSensorValues();
    var bytes = [];
    var battery = getState('battery') || 100;
    var tvocMode = getState('tvocMode') || 0;

    // Battery Level: Channel 0x01, Type 0x75
    bytes.push(0x01, 0x75, battery & 0xFF);

    // Temperature: Channel 0x03, Type 0x67 (INT16/10, °C)
    bytes.push(0x03, 0x67);
    bytes = bytes.concat(int16LE(Math.round(values.temperature * 10)));

    // Humidity: Channel 0x04, Type 0x68 (UINT8/2, %RH)
    bytes.push(0x04, 0x68, Math.round(values.humidity * 2) & 0xFF);

    // PIR Status: Channel 0x05, Type 0x00
    bytes.push(0x05, 0x00, values.pir ? 0x01 : 0x00);

    // Light Level: Channel 0x06, Type 0xCB
    bytes.push(0x06, 0xCB, values.lightLevel & 0xFF);

    // CO2: Channel 0x07, Type 0x7D (UINT16, ppm)
    bytes.push(0x07, 0x7D);
    bytes = bytes.concat(uint16LE(values.co2));

    // TVOC: Channel 0x08
    if (tvocMode === 0) {
        bytes.push(0x08, 0x7D);
        bytes = bytes.concat(uint16LE(Math.round(values.tvocLevel * 100)));
    } else {
        bytes.push(0x08, 0xE6);
        bytes = bytes.concat(uint16LE(values.tvocConcentration));
    }

    // Barometric Pressure: Channel 0x09, Type 0x73
    bytes.push(0x09, 0x73);
    bytes = bytes.concat(uint16LE(Math.round(values.pressure * 10)));

    // HCHO: Channel 0x0A, Type 0x7D
    bytes.push(0x0A, 0x7D);
    bytes = bytes.concat(uint16LE(Math.round(values.hcho * 100)));

    // PM2.5: Channel 0x0B, Type 0x7D
    bytes.push(0x0B, 0x7D);
    bytes = bytes.concat(uint16LE(values.pm25));

    // PM10: Channel 0x0C, Type 0x7D
    bytes.push(0x0C, 0x7D);
    bytes = bytes.concat(uint16LE(values.pm10));

    log('AM319 Uplink: Temp=' + values.temperature.toFixed(1) + '°C, Hum=' +
        values.humidity.toFixed(1) + '%, CO2=' + values.co2 + 'ppm');

    return { fPort: 85, bytes: bytes };
}

function OnDownlink(bytes, fPort) {
    initState();

    log('fPort=' + fPort + ', bytes=' + bytesToHex(bytes));

    var i = 0;
    while (i < bytes.length) {
        if (bytes[i] !== 0xFF) { i++; continue; }
        var cmd = bytes[i + 1];
        switch (cmd) {
            case 0x03: // Report interval
                var intervalSec = decodeUint16LE(bytes, i + 2);
                setSendInterval(intervalSec);
                log('Interval set to ' + intervalSec + 's');
                i += 4;
                break;
            case 0xEB: // TVOC mode
                var tvocMode = bytes[i + 2];
                setState('tvocMode', tvocMode);
                i += 3;
                break;
            case 0x1A: // CO2 calibration
                if (bytes[i + 2] === 0x03) setState('baseCO2', 400);
                i += 3;
                break;
            default:
                i += 3;
                break;
        }
    }
}

// Converts a byte array to a lowercase hex string without spaces
// Example: [255, 3, 30, 0] -> "ff031e00"
// Handles undefined/null bytes by treating them as 0
function bytesToHex(bytes) {
    var hex = '';
    for (var i = 0; i < bytes.length; i++) {
        var byte = bytes[i];
        if (byte === undefined || byte === null) byte = 0;
        hex += ('0' + (byte & 0xFF).toString(16)).slice(-2);
    }
    return hex;
}
`
}
