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
// Milesight AM319 - Indoor Ambiance Monitoring Sensor
// https://www.milesight.com/iot/product/lorawan-sensor/am319
// Uplink fPort 85, Milesight channel format (DC powered, no battery)

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
    var baseTemp = getState('baseTemperature') || 19.0;
    var baseHumidity = getState('baseHumidity') || 35;
    var baseCO2 = getState('baseCO2') || 450;
    var tvocMode = getState('tvocMode') || 0;

    var values = {};
    values.temperature = baseTemp + (Math.random() - 0.5) * 2;
    values.humidity = baseHumidity + (Math.random() - 0.5) * 10;
    values.pir = Math.random() > 0.9 ? 1 : 0;
    values.lightLevel = Math.floor(Math.random() * 6);
    values.co2 = Math.max(400, Math.floor(baseCO2 + (Math.random() - 0.5) * 100));
    values.pressure = 990 + (Math.random() - 0.5) * 10;
    values.hcho = Math.max(0.01, 0.02 + (Math.random() - 0.5) * 0.02);
    values.pm25 = Math.max(0, Math.floor(12 + (Math.random() - 0.5) * 10));
    values.pm10 = Math.max(0, Math.floor(12 + (Math.random() - 0.5) * 10));

    if (tvocMode === 0) {
        values.tvocLevel = 0.7 + (Math.random() - 0.5) * 0.2;
    } else {
        values.tvocConcentration = Math.floor(200 + (Math.random() - 0.5) * 100);
    }

    return values;
}

function OnUplink() {
    var values = generateSensorValues();
    var bytes = [];
    var tvocMode = getState('tvocMode') || 0;

    bytes.push(0x03, 0x67);
    bytes = bytes.concat(int16LE(Math.round(values.temperature * 10)));

    bytes.push(0x04, 0x68, Math.round(values.humidity * 2) & 0xFF);

    bytes.push(0x05, 0x00, values.pir ? 0x01 : 0x00);

    bytes.push(0x06, 0xCB, values.lightLevel & 0xFF);

    bytes.push(0x07, 0x7D);
    bytes = bytes.concat(uint16LE(values.co2));

    if (tvocMode === 0) {
        bytes.push(0x08, 0x7D);
        bytes = bytes.concat(uint16LE(Math.round(values.tvocLevel * 100)));
    } else {
        bytes.push(0x08, 0xE6);
        bytes = bytes.concat(uint16LE(values.tvocConcentration));
    }

    bytes.push(0x09, 0x73);
    bytes = bytes.concat(uint16LE(Math.round(values.pressure * 10)));

    bytes.push(0x0A, 0x7D);
    bytes = bytes.concat(uint16LE(Math.round(values.hcho * 100)));

    bytes.push(0x0B, 0x7D);
    bytes = bytes.concat(uint16LE(values.pm25));

    bytes.push(0x0C, 0x7D);
    bytes = bytes.concat(uint16LE(values.pm10));

    log('TX | Temp=' + values.temperature.toFixed(1) + 'C Hum=' +
        values.humidity.toFixed(1) + '% CO2=' + values.co2 + 'ppm');

    return { fPort: 85, bytes: bytes };
}

function OnDownlink(bytes, fPort) {
    log('RX | fPort=' + fPort + ' data=' + bytesToHex(bytes));

    var i = 0;
    while (i < bytes.length) {
        if (bytes[i] !== 0xFF) { i++; continue; }
        var cmd = bytes[i + 1];
        switch (cmd) {
            case 0x03:
                var intervalSec = decodeUint16LE(bytes, i + 2);
                setSendInterval(intervalSec);
                log('Set report interval: ' + intervalSec + 's');
                i += 4;
                break;
            case 0xEB:
                setState('tvocMode', bytes[i + 2]);
                i += 3;
                break;
            case 0x1A:
                if (bytes[i + 2] === 0x03) setState('baseCO2', 400);
                i += 3;
                break;
            default:
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

    // Clear any existing timer when output state changes
    if (enableMask & 0x01 || disableMask & 0x01) {
        setState('timer', null);
    }

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

// CreateSDM230Codec returns the Eastron SDM230 energy meter codec script
func CreateSDM230Codec() string {
	return `
// ============================================================================
// Eastron SDM230 - LoRaWAN Single-Phase Energy Meter Codec
// ============================================================================
//
// Device: SDM230-LoRaWAN (single-phase energy meter)
// Class:  LoRaWAN Class C
// Docs:   Eastron SDM230-LoRaWAN Protocol V1.1
//
// UPLINK (fPort 1): 28-byte fixed payload (verified against real device)
//   Bytes 0-3:   Serial Number (uint32 Little-Endian)
//   Byte 4:      Message Fragment Number (always 1)
//   Byte 5:      Number of Parameter Bytes (0x14 = 20 for 5 floats)
//   Bytes 6-9:   Active Energy Total (float32 Big-Endian, kWh)
//   Bytes 10-13: Voltage (float32 Big-Endian, V)
//   Bytes 14-17: Current (float32 Big-Endian, A)
//   Bytes 18-21: Power Factor (float32 Big-Endian, 0-1)
//   Bytes 22-25: Frequency (float32 Big-Endian, Hz)
//   Bytes 26-27: Modbus CRC-16 (Little-Endian)
//
// DOWNLINK (fPort 1): Modbus RTU commands
//   Set interval:  01 10 FE 01 00 01 02 <min_hi> <min_lo> <crc_lo> <crc_hi>
//   Set params:    01 10 FE 02 00 0F 1E <30 param bytes> <crc_lo> <crc_hi>
//   Reset energy:  01 10 F0 10 00 01 02 00 03 <crc_lo> <crc_hi>
//
// ============================================================================

function initState() {
    // Default serial number matches real device: 0x8bfb590e
    if (getState('serialNumber') === null) setState('serialNumber', 0x8bfb590e);
    // Default energy ~745 kWh to match real device logs
    if (getState('energy') === null) setState('energy', 744.5 + Math.random() * 1);
    // Default values from real device measurements
    if (getState('baseVoltage') === null) setState('baseVoltage', 233.5);
    if (getState('baseCurrent') === null) setState('baseCurrent', 0.528);
    if (getState('basePowerFactor') === null) setState('basePowerFactor', 0.9765);
}

function bytesToHex(bytes) {
    var hex = '';
    for (var i = 0; i < bytes.length; i++) {
        hex += ('0' + ((bytes[i] || 0) & 0xFF).toString(16)).slice(-2);
    }
    return hex;
}

// Convert uint32 to 4 bytes (Little-Endian - verified from real device)
function uint32ToLE(value) {
    return [
        value & 0xFF,
        (value >> 8) & 0xFF,
        (value >> 16) & 0xFF,
        (value >> 24) & 0xFF
    ];
}

// Convert float32 to 4 bytes (Big-Endian) using IEEE 754
function floatToBytes(value) {
    var buffer = new ArrayBuffer(4);
    var view = new DataView(buffer);
    view.setFloat32(0, value, false); // false = big-endian
    return [view.getUint8(0), view.getUint8(1), view.getUint8(2), view.getUint8(3)];
}

// Calculate Modbus CRC-16
function modbusCRC16(bytes) {
    var crc = 0xFFFF;
    for (var i = 0; i < bytes.length; i++) {
        crc ^= bytes[i];
        for (var j = 0; j < 8; j++) {
            if (crc & 0x0001) {
                crc = (crc >> 1) ^ 0xA001;
            } else {
                crc = crc >> 1;
            }
        }
    }
    return [crc & 0xFF, (crc >> 8) & 0xFF];
}

// Decode uint16 from bytes (Big-Endian)
function uint16BE(bytes, offset) {
    return (bytes[offset] << 8) | bytes[offset + 1];
}

function generateMeterReadings() {
    var readings = {};
    var baseVoltage = getState('baseVoltage') || 233.5;
    var baseCurrent = getState('baseCurrent') || 0.528;
    var basePF = getState('basePowerFactor') || 0.9765;

    // Simulate energy accumulation
    var energy = getState('energy') || 744.5;
    var sendInterval = getSendInterval() || 1800; // default 30 min (matches real device)

    // Calculate energy increment based on power consumption
    // Real device shows ~0.06 kWh increase per 30 minutes
    var activePower = baseCurrent * baseVoltage * basePF; // ~120W
    var energyIncrement = (activePower * sendInterval) / 3600000; // kWh
    energy += energyIncrement + (Math.random() - 0.5) * 0.01;
    setState('energy', energy);

    // Generate readings with realistic variations matching real device
    readings.energy = energy;
    readings.voltage = baseVoltage + (Math.random() - 0.5) * 3;      // ~231-235V range
    readings.current = baseCurrent + (Math.random() - 0.5) * 0.004;  // ~0.526-0.530A
    readings.powerFactor = Math.min(1, Math.max(0.97, basePF + (Math.random() - 0.5) * 0.002));
    readings.frequency = 49.95 + Math.random() * 0.1;                // 49.95-50.05 Hz

    return readings;
}

function OnUplink() {
    initState();
    var readings = generateMeterReadings();
    var serialNumber = getState('serialNumber') || 0x8bfb590e;

    var bytes = [];

    // Serial Number (4 bytes, Little-Endian - verified from real device)
    bytes = bytes.concat(uint32ToLE(serialNumber));

    // Message Fragment Number (always 1)
    bytes.push(0x01);

    // Number of Parameter Bytes (0x14 = 20 bytes for 5 floats)
    bytes.push(0x14);

    // Active Energy Total (float32 Big-Endian, kWh)
    bytes = bytes.concat(floatToBytes(readings.energy));

    // Voltage (float32 Big-Endian, V)
    bytes = bytes.concat(floatToBytes(readings.voltage));

    // Current (float32 Big-Endian, A)
    bytes = bytes.concat(floatToBytes(readings.current));

    // Power Factor (float32 Big-Endian)
    bytes = bytes.concat(floatToBytes(readings.powerFactor));

    // Frequency (float32 Big-Endian, Hz)
    bytes = bytes.concat(floatToBytes(readings.frequency));

    // Modbus CRC-16 (2 bytes, Little-Endian)
    var crc = modbusCRC16(bytes);
    bytes = bytes.concat(crc);

    log('TX SDM230 | E=' + readings.energy.toFixed(2) + 'kWh V=' +
        readings.voltage.toFixed(1) + 'V I=' + readings.current.toFixed(3) + 'A PF=' +
        readings.powerFactor.toFixed(4) + ' F=' + readings.frequency.toFixed(2) + 'Hz');

    return { fPort: 1, bytes: bytes };
}

function OnDownlink(bytes, fPort) {
    initState();
    log('RX SDM230 | fPort=' + fPort + ' data=' + bytesToHex(bytes));

    if (bytes.length < 4) {
        log('  Error: Payload too short');
        return;
    }

    // Verify CRC
    var dataLen = bytes.length - 2;
    var expectedCrc = modbusCRC16(bytes.slice(0, dataLen));
    if (bytes[dataLen] !== expectedCrc[0] || bytes[dataLen + 1] !== expectedCrc[1]) {
        log('  Warning: CRC mismatch (continuing anyway)');
    }

    // Parse Modbus RTU command
    var slaveAddr = bytes[0];   // Usually 0x01
    var funcCode = bytes[1];    // 0x10 = write, 0x03 = read

    if (funcCode === 0x10) {
        // Write multiple registers
        var regAddr = uint16BE(bytes, 2);
        var regCount = uint16BE(bytes, 4);
        var byteCount = bytes[6];

        log('  Modbus Write: addr=0x' + regAddr.toString(16) + ' regs=' + regCount);

        if (regAddr === 0xFE01) {
            // Set upload interval (minutes)
            var intervalMin = uint16BE(bytes, 7);
            var intervalSec = intervalMin * 60;
            setSendInterval(intervalSec);
            log('  Set interval: ' + intervalMin + ' min (' + intervalSec + 's)');

        } else if (regAddr === 0xF010) {
            // Reset commands
            var resetType = uint16BE(bytes, 7);
            if (resetType === 0x0000) {
                log('  Reset maximum demand');
            } else if (resetType === 0x0003) {
                setState('energy', 0);
                log('  Reset resettable energy counter');
            }
        } else {
            log('  Unknown register: 0x' + regAddr.toString(16));
        }
    } else if (funcCode === 0x03) {
        // Read registers (authentication/status check)
        log('  Modbus Read request (acknowledged)');
    } else {
        log('  Unknown function code: 0x' + funcCode.toString(16));
    }
}
`
}
