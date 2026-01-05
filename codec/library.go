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
// ============================================================================
// Milesight AM319 - Indoor Ambiance Monitoring Sensor Codec
// ============================================================================
//
// Device: AM319 (temperature, humidity, CO2, TVOC, PM2.5, PM10, HCHO, pressure)
// Docs:   https://www.milesight.com/iot/product/lorawan-sensor/am319
//
// UPLINK (fPort 85): Sensor readings in Milesight channel format
// DOWNLINK (fPort 85): Configuration commands (0xFF prefix)
//
// DOWNLINK EXAMPLES:
//   Set report interval to 300s:  FF 03 2C 01
//   Set TVOC mode to ppb:         FF EB 01
//   Calibrate CO2 to 400ppm:      FF 1A 03
//
// ============================================================================

function initState() {
    if (getState('battery') === null) setState('battery', 100);
    if (getState('tvocMode') === null) setState('tvocMode', 0);
}

function bytesToHex(bytes) {
    var hex = '';
    for (var i = 0; i < bytes.length; i++) {
        hex += ('0' + ((bytes[i] || 0) & 0xFF).toString(16)).slice(-2);
    }
    return hex;
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

    log('TX Sensor Data | Temp=' + values.temperature.toFixed(1) + 'C Hum=' +
        values.humidity.toFixed(1) + '% CO2=' + values.co2 + 'ppm PIR=' + values.pir);

    return { fPort: 85, bytes: bytes };
}

function OnDownlink(bytes, fPort) {
    initState();
    log('RX Downlink | fPort=' + fPort + ' data=' + bytesToHex(bytes));

    var i = 0;
    while (i < bytes.length) {
        if (bytes[i] !== 0xFF) { i++; continue; }
        var cmd = bytes[i + 1];
        switch (cmd) {
            case 0x03: // Report interval
                var intervalSec = decodeUint16LE(bytes, i + 2);
                setSendInterval(intervalSec);
                log('  Set report interval: ' + intervalSec + 's');
                i += 4;
                break;
            case 0xEB: // TVOC mode
                var tvocMode = bytes[i + 2];
                setState('tvocMode', tvocMode);
                log('  Set TVOC mode: ' + (tvocMode === 0 ? 'level' : 'ppb'));
                i += 3;
                break;
            case 0x1A: // CO2 calibration
                if (bytes[i + 2] === 0x03) {
                    setState('baseCO2', 400);
                    log('  CO2 calibrated to 400ppm');
                }
                i += 3;
                break;
            default:
                log('  Unknown command: 0x' + cmd.toString(16));
                i += 3;
                break;
        }
    }
}
`
}

// CreateMCFLW13IOCodec returns the Enginko MCF-LW13IO I/O Controller codec script
func CreateMCFLW13IOCodec() string {
	return `
// ============================================================================
// Enginko MCF-LW13IO - LoRaWAN I/O Controller Codec
// ============================================================================
//
// Device: MCF-LW13IO (1 relay output, 1 digital input)
// Class:  LoRaWAN Class C (always listening)
// Docs:   https://www.enginko.com/support/doku.php?id=manual_mcf-lw13io
//
// UPLINK MESSAGES (fPort 2):
//   0x01 - TimeSync: Device info, firmware version
//   0x0A - IO Status: Input state, output state, trigger events
//
// DOWNLINK FORMAT (fPort 2):
//   0x04 0x00 <enable 4B> <disable 4B> [Ton 2B]
//
// DOWNLINK EXAMPLES:
//   Turn ON output 1:            04 00 01000000 00000000
//   Turn OFF output 1:           04 00 00000000 01000000
//   Turn ON output 1 for 12.3s:  04 00 01000000 00000000 7B00
//   Turn OFF output 1 for 12.3s: 04 00 00000000 01000000 7B00
//
// ============================================================================

function initState() {
    if (getState('input') === null) setState('input', false);
    if (getState('output') === null) setState('output', false);
    if (getState('trigger') === null) setState('trigger', false);
    if (getState('messageCount') === null) setState('messageCount', 0);
    if (getState('syncID') === null) setState('syncID', 0x2B438900);
}

function bytesToHex(bytes) {
    var hex = '';
    for (var i = 0; i < bytes.length; i++) {
        hex += ('0' + ((bytes[i] || 0) & 0xFF).toString(16)).slice(-2);
    }
    return hex;
}

// Encode Enginko date: year(7b) month(4b) day(5b) hour(5b) min(6b) sec/2(5b)
function encodeDate() {
    var now = new Date();
    var packed = ((now.getFullYear() - 2000) & 0x7F) << 25;
    packed |= ((now.getMonth() + 1) & 0x0F) << 21;
    packed |= (now.getDate() & 0x1F) << 16;
    packed |= (now.getHours() & 0x1F) << 11;
    packed |= (now.getMinutes() & 0x3F) << 5;
    packed |= (Math.floor(now.getSeconds() / 2) & 0x1F);
    return [packed & 0xFF, (packed >> 8) & 0xFF, (packed >> 16) & 0xFF, (packed >> 24) & 0xFF];
}

function uint32LE(bytes, offset) {
    return bytes[offset] | (bytes[offset+1] << 8) | (bytes[offset+2] << 16) | (bytes[offset+3] << 24);
}

function uint16LE(bytes, offset) {
    return bytes[offset] | (bytes[offset+1] << 8);
}

// Check and process expired timer
function processTimer() {
    var timer = getState('timer');
    if (timer && Date.now() >= timer.expiry) {
        setState('output', timer.action === 'on');
        setState('timer', null);
        log('Timer expired: OUT1 -> ' + (timer.action === 'on' ? 'ON' : 'OFF'));
    }
}

function generateTimeSync() {
    var syncID = (getState('syncID') + 1) & 0xFFFFFFFF;
    setState('syncID', syncID);

    var bytes = [0x01];
    bytes.push(syncID & 0xFF, (syncID >> 8) & 0xFF, (syncID >> 16) & 0xFF, (syncID >> 24) & 0xFF);
    bytes.push(0x00, 0x02, 0x5d); // Firmware 0.2.93
    bytes.push(0x07, 0x01);       // App type (I/O controller)
    bytes.push(0x00);             // RFU

    log('TX TimeSync | syncID=' + syncID.toString(16) + ' firmware=0.2.93');
    return bytes;
}

function generateIOStatus() {
    processTimer();

    var input = getState('input');
    var output = getState('output');
    var trigger = getState('trigger');

    // Simulate random input activity (~30% chance)
    if (Math.random() > 0.7) {
        input = !input;
        trigger = true;
        setState('input', input);
        setState('trigger', trigger);
    }

    var bytes = [0x0A];
    bytes = bytes.concat(encodeDate());
    // Input (4 bytes LE) - bit 0 only
    bytes.push(input ? 0x01 : 0x00, 0x00, 0x00, 0x00);
    // Output (4 bytes LE) - bit 0 only
    bytes.push(output ? 0x01 : 0x00, 0x00, 0x00, 0x00);
    // Trigger (4 bytes LE) - bit 0 only
    bytes.push(trigger ? 0x01 : 0x00, 0x00, 0x00, 0x00);

    setState('trigger', false);

    log('TX IO Status | IN1=' + (input ? 'ON' : 'OFF') + ' OUT1=' + (output ? 'ON' : 'OFF') + ' TRIG=' + (trigger ? 'yes' : 'no'));
    return bytes;
}

function OnUplink() {
    initState();
    var count = (getState('messageCount') || 0) + 1;
    setState('messageCount', count);

    // Send TimeSync every 10th message
    var bytes = (count % 10 === 1) ? generateTimeSync() : generateIOStatus();
    return { fPort: 2, bytes: bytes };
}

function OnDownlink(bytes, fPort) {
    initState();
    log('RX Downlink | fPort=' + fPort + ' data=' + bytesToHex(bytes));

    if (bytes.length < 10) {
        log('  Error: Payload too short');
        return;
    }

    if (bytes[0] !== 0x04 || bytes[1] !== 0x00) {
        log('  Error: Unknown command');
        return;
    }

    var enableMask = uint32LE(bytes, 2);
    var disableMask = uint32LE(bytes, 6);
    var oldOutput = getState('output');
    var newOutput = oldOutput;

    // Apply immediate state change
    if (enableMask & 0x01) newOutput = true;
    if (disableMask & 0x01) newOutput = false;
    setState('output', newOutput);

    log('  OUT1: ' + (oldOutput ? 'ON' : 'OFF') + ' -> ' + (newOutput ? 'ON' : 'OFF'));

    // Parse Ton timer if present
    if (bytes.length >= 12 && (enableMask & 0x01 || disableMask & 0x01)) {
        var tonValue = uint16LE(bytes, 10);
        if (tonValue > 0) {
            var action = (enableMask & 0x01) ? 'off' : 'on';
            setState('timer', { expiry: Date.now() + tonValue * 100, action: action });
            log('  Timer: OUT1 -> ' + action.toUpperCase() + ' in ' + (tonValue / 10) + 's');
        }
    }
}
`
}
