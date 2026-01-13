/**
 * Codec Management JavaScript
 * Handles UI for creating, editing, and deleting custom JavaScript codecs
 */

"use strict";

var Codecs = new Map();

// Wrapper functions that call the global functions set up by monaco-init.js
function GetCodecEditorValue() {
    if (window.GetMonacoEditorValue) {
        return window.GetMonacoEditorValue();
    }
    return $("#textarea-codec-script").val();
}

function SetCodecEditorValue(value) {
    if (window.SetMonacoEditorValue) {
        window.SetMonacoEditorValue(value);
    } else {
        $("#textarea-codec-script").val(value);
    }
}

function SetCodecEditorReadOnly(readOnly) {
    if (window.SetMonacoEditorReadOnly) {
        window.SetMonacoEditorReadOnly(readOnly);
    } else {
        $("#textarea-codec-script").prop("disabled", readOnly);
    }
}

// Load codecs from API and populate the list
function LoadCodecList() {
    $.ajax({
        url: url+"/api/codecs",
        type:"GET",
        headers:{
            "Access-Control-Allow-Origin":"*"
        }
    }).done((data)=>{
        $("#list-codecs").empty();
        Codecs.clear();

        if(data.codecs && data.codecs.length > 0){
            // Load all codecs and their usage data
            data.codecs.forEach(codec => {
                Codecs.set(codec.id, codec);
                // Fetch usage data for each codec
                $.get(url + "/api/codec/" + codec.id + "/usage")
                    .done((usageData) => {
                        AddItemListCodecs(codec, usageData);
                    })
                    .fail(() => {
                        AddItemListCodecs(codec, null);
                    });
            });
        }
    }).fail((data)=>{
        console.error("Unable to load codecs", data.statusText);
    });
}

// Add codec item to the list
function AddItemListCodecs(codec, usageData){
    var usageText = "Not used";

    if (usageData && usageData.count > 0) {
        // Count devices and templates separately
        var deviceCount = 0;
        var templateCount = 0;

        usageData.devices.forEach(function(item) {
            if (item.startsWith("template:")) {
                templateCount++;
            } else {
                deviceCount++;
            }
        });

        var parts = [];
        if (deviceCount > 0) {
            parts.push(deviceCount + " device" + (deviceCount > 1 ? "s" : ""));
        }
        if (templateCount > 0) {
            parts.push(templateCount + " template" + (templateCount > 1 ? "s" : ""));
        }
        usageText = parts.join(", ");
    }

    var item = "<tr data-id=\""+codec.id+"\">\
                    <td class=\"clickable font-weight-bold font-italic text-navy\" >"+codec.name+"</td>\
                    <td>"+usageText+"</td>\
                </tr>";

    $("#list-codecs").append(item);
}

// Handle codec list item click - load codec for editing
$(document).on('click', '#list-codecs tr', function(){
    var id = parseInt($(this).data("id"));
    var codecMeta = Codecs.get(id);

    if(!codecMeta){
        return;
    }

    // Fetch full codec details including script
    $.ajax({
        url: url+"/api/codec/"+id,
        type:"GET",
        headers:{
            "Access-Control-Allow-Origin":"*"
        }
    }).done((data)=>{
        if(data.codec){
            LoadCodec(data.codec);
            ShowList($("#add-codec"),"Edit Codec",false);
        }
    }).fail((data)=>{
        Show_ErrorSweetToast("Unable to load codec details", data.statusText);
    });
});

// Load codec data into the form
function LoadCodec(codec){
    $("[name=input-codec-name]").val(codec.name);
    SetCodecEditorValue(codec.script);

    // Set all fields to disabled (view-only mode)
    $("[name=input-codec-name]").prop("disabled", true);
    SetCodecEditorReadOnly(true);

    $("#div-buttons-codec").data("id", codec.id);
    $("[name=btn-delete-codec]").show();
    $("[name=btn-edit-codec]").show();
    $("[name=btn-save-codec]").hide();

    $("#codecs").removeClass("show active");
    $("#add-codec").addClass("show active");
    $(".section-header h1").text("Edit Codec");
}

// Clean codec form
function CleanCodecForm(){
    $("[name=input-codec-name]").val("");

    // Set skeleton code for new codec
    var skeletonCode = `// OnUplink function: Called when device sends an uplink
// Returns byte array or {fPort: number, bytes: array}
// Available helpers: getState, setState, getSendInterval, setSendInterval, hexToBytes, base64ToBytes, log
function OnUplink() {
    var bytes = [];

    // Example: Stateful counter
    var counter = getState('counter') || 0;
    setState('counter', counter + 1);

    // Example: Encode temperature and humidity
    // Temperature: 2 bytes (signed int16, resolution 0.1°C)
    var temp = Math.round(20 * 10);  // 20°C
    bytes.push((temp >> 8) & 0xFF);
    bytes.push(temp & 0xFF);

    // Humidity: 1 byte (unsigned int, 0-100%)
    var humidity = 50;  // 50%
    bytes.push(humidity & 0xFF);

    // Counter: 2 bytes
    bytes.push((counter >> 8) & 0xFF);
    bytes.push(counter & 0xFF);

    // Option 1: Return bytes with custom fPort
    return {
        fPort: 85,
        bytes: bytes
    };

    // Option 2: Return bytes only (uses device's configured fPort)
    // return bytes;

    // Option 3: Use hexToBytes for readable hex strings
    // return hexToBytes("0367e50468420500");

    // Option 4: Use base64ToBytes for base64 strings
    // return base64ToBytes("A2fEAARoQgUA");
}

// OnDownlink function (OPTIONAL): Called when device receives a downlink
// Executed for side effects only (log, setState, setSendInterval)
// No return value needed
function OnDownlink(bytes, fPort) {
    log('Downlink: fPort=' + fPort + ', bytes=' + bytes.length);

    // Example: Handle downlink commands
    if (bytes.length >= 3 && bytes[0] === 0x01) {
        // Command 0x01: Set send interval (bytes 1-2 = interval in seconds)
        var interval = (bytes[1] << 8) | bytes[2];
        log('Setting interval to ' + interval + 's');
        setSendInterval(interval);
    }

    // Example: Store received data in state
    if (bytes.length >= 2) {
        setState('lastDownlink', bytes);
    }
}`;

    SetCodecEditorValue(skeletonCode);

    // Enable all fields for new codec
    $("[name=input-codec-name]").prop("disabled", false);
    SetCodecEditorReadOnly(false);

    $("#div-buttons-codec").removeData("id");
    $("[name=btn-delete-codec]").hide();
    $("[name=btn-edit-codec]").hide();
    $("[name=btn-save-codec]").show();
}

// Save codec button click
$("[name=btn-save-codec]").on('click', function(){
    SaveCodec(false);
});

// Edit codec button click
$("[name=btn-edit-codec]").on('click', function(){
    $(this).hide();
    $("[name=btn-delete-codec]").hide();
    $("[name=btn-save-codec]").show();

    $("[name=input-codec-name]").prop("disabled", false);
    SetCodecEditorReadOnly(false);
});

// Delete codec button click
$("[name=btn-delete-codec]").on('click', function(){
    var codecId = $("#div-buttons-codec").data("id");

    if(!codecId){
        Show_ErrorSweetToast("Error", "No codec selected");
        return;
    }

    swal({
        title: "Are you sure?",
        text: "Once deleted, you will not be able to recover this codec!",
        icon: "warning",
        buttons: true,
        dangerMode: true,
    })
    .then((willDelete) => {
        if (willDelete) {
            DeleteCodec(codecId);
        }
    });
});

// Save codec function
function SaveCodec(isUpdate){
    var name = $("[name=input-codec-name]").val();
    var script = GetCodecEditorValue();
    var codecId = $("#div-buttons-codec").data("id");

    if(!name || !script){
        Show_ErrorSweetToast("Error", "Name and Script are required");
        return;
    }

    // Determine if we're editing (codecId exists) or adding (no codecId)
    var isEdit = codecId && codecId !== 0;
    var apiEndpoint = isEdit ? "/api/update-codec" : "/api/add-codec";

    var codecData = {
        "name": name,
        "script": script
    };

    // Add ID if editing
    if(isEdit){
        codecData.id = codecId;
    }

    var jsonData = JSON.stringify(codecData);

    $.post(url + apiEndpoint, jsonData, "json")
    .done((data)=>{
        Show_SweetToast(isEdit ? "Codec updated successfully" : "Codec saved successfully", "");
        CleanCodecForm();
        LoadCodecList();
        PopulatePayloadGenerationDropdown();
        ShowList($("#codecs"),"List codecs",false);
    })
    .fail((data)=>{
        var errorMsg = data.responseJSON ? data.responseJSON.error : data.statusText;
        Show_ErrorSweetToast(isEdit ? "Failed to update codec" : "Failed to save codec", errorMsg);
    });
}

// Delete codec function
function DeleteCodec(codecId){
    // First check if codec is in use
    $.get(url + "/api/codec/" + codecId + "/usage")
    .done((usageData) => {
        if (usageData.count > 0) {
            // Count devices and templates separately
            var deviceCount = 0;
            var templateCount = 0;

            usageData.devices.forEach(function(item) {
                if (item.startsWith("template:")) {
                    templateCount++;
                } else {
                    deviceCount++;
                }
            });

            var parts = [];
            if (deviceCount > 0) {
                parts.push(deviceCount + " device" + (deviceCount > 1 ? "s" : ""));
            }
            if (templateCount > 0) {
                parts.push(templateCount + " template" + (templateCount > 1 ? "s" : ""));
            }
            var usageText = parts.join(" and ");

            // Show warning with device and template count
            swal({
                title: "Cannot Delete Codec",
                text: "This codec is currently used by " + usageText + ". Please remove the codec from all devices and templates before deleting.",
                icon: "warning",
                button: "OK"
            });
        } else {
            // Confirm deletion
            swal({
                title: "Are you sure?",
                text: "Once deleted, this codec cannot be recovered.",
                icon: "warning",
                buttons: true,
                dangerMode: true,
            })
            .then((willDelete) => {
                if (willDelete) {
                    var jsonData = JSON.stringify({"id": codecId});
                    $.post(url + "/api/delete-codec", jsonData, "json")
                    .done((data)=>{
                        Show_SweetToast("Codec deleted successfully", "");
                        CleanCodecForm();
                        LoadCodecList();
                        PopulatePayloadGenerationDropdown();
                        ShowList($("#codecs"),"List codecs",false);
                    })
                    .fail((data)=>{
                        var errorMsg = data.responseJSON ? data.responseJSON.error : data.statusText;
                        Show_ErrorSweetToast("Failed to delete codec", errorMsg);
                    });
                }
            });
        }
    })
    .fail((data) => {
        Show_ErrorSweetToast("Failed to check codec usage", data.statusText);
    });
}

// Initialize codec list on page load
$("#codecs-tab").on('click', function(){
    LoadCodecList();
    $(".section-header h1").text("List Codecs");
});

// Clean form when adding new codec
$("#add-codec-tab").on('click', function(){
    CleanCodecForm();
    $(".section-header h1").text("Add New Codec");
});

// Initialize Monaco Editor when DOM is ready
$(document).ready(function(){
    // Initialize editor when the add-codec tab is first shown
    $("#add-codec-tab, #codecs-tab").one('click', function(){
        setTimeout(function() {
            if (window.InitializeMonacoEditor) {
                window.InitializeMonacoEditor();
            }
        }, 100);
    });
});

/**
 * ========================================
 * Browser-Based Codec Testing
 * ========================================
 * Provides client-side execution environment for testing codec functions
 */

// Session-only test state (cleared on page refresh)
var CodecTestState = {
    // Persistent state storage (mimics backend state)
    state: {},

    // Mock send interval (seconds)
    sendInterval: 300,

    // Log entries with timestamps
    logs: [],

    // Add log entry
    addLog: function(message, type) {
        type = type || 'info'; // 'info', 'error', 'warn'
        var timestamp = new Date().toLocaleTimeString();
        this.logs.push({
            timestamp: timestamp,
            message: message,
            type: type
        });
        this.renderLogs();
    },

    // Render logs to console output
    renderLogs: function() {
        var logsContainer = $("#output-console-logs");
        logsContainer.empty();

        if (this.logs.length === 0) {
            logsContainer.html('<div class="text-muted">Console output will appear here...</div>');
            return;
        }

        this.logs.forEach(function(log) {
            var color = log.type === 'error' ? 'text-danger' :
                        log.type === 'warn' ? 'text-warning' :
                        'text-white';
            var logLine = $('<div class="' + color + '"></div>');
            logLine.text('[' + log.timestamp + '] ' + log.message);
            logsContainer.append(logLine);
        });

        // Auto-scroll to bottom
        logsContainer.scrollTop(logsContainer[0].scrollHeight);
    },

    // Render state viewer
    renderState: function() {
        var stateJson = JSON.stringify(this.state, null, 2);
        $("#output-state-viewer").text(stateJson);
    },

    // Reset all test state
    reset: function() {
        this.state = {};
        this.sendInterval = 300;
        this.logs = [];
        this.renderState();
        this.renderLogs();
    },

    // Clear logs only
    clearLogs: function() {
        this.logs = [];
        this.renderLogs();
    }
};

/**
 * Create browser-compatible helper functions
 * These mimic the backend helpers injected by codec/library.go
 */
function CreateCodecHelpers() {
    return {
        // State management
        getState: function(key) {
            var value = CodecTestState.state[key];
            return value !== undefined ? value : null;
        },

        setState: function(key, value) {
            CodecTestState.state[key] = value;
            CodecTestState.renderState();
        },

        // Send interval management
        getSendInterval: function() {
            return CodecTestState.sendInterval;
        },

        setSendInterval: function(seconds) {
            CodecTestState.sendInterval = parseInt(seconds);
            CodecTestState.addLog('Send interval set to ' + seconds + 's', 'info');
        },

        // Hex to bytes conversion
        hexToBytes: function(hexString) {
            // Remove spaces and validate hex
            hexString = hexString.replace(/\s+/g, '');
            if (!/^[0-9A-Fa-f]*$/.test(hexString)) {
                throw new Error("Invalid hex string");
            }
            if (hexString.length % 2 !== 0) {
                throw new Error("Hex string must have even length");
            }

            var bytes = [];
            for (var i = 0; i < hexString.length; i += 2) {
                bytes.push(parseInt(hexString.substr(i, 2), 16));
            }
            return bytes;
        },

        // Base64 to bytes conversion
        base64ToBytes: function(b64String) {
            try {
                var binaryString = atob(b64String);
                var bytes = [];
                for (var i = 0; i < binaryString.length; i++) {
                    bytes.push(binaryString.charCodeAt(i));
                }
                return bytes;
            } catch (e) {
                throw new Error("Invalid base64 string: " + e.message);
            }
        },

        // Logging
        log: function(message) {
            CodecTestState.addLog(String(message), 'info');
        }
    };
}

/**
 * Execute codec function in isolated context
 * Uses Function constructor with controlled scope to prevent access to window/document
 */
function ExecuteCodecFunction(codecScript, functionName, args) {
    var startTime = performance.now();
    var result = null;
    var error = null;

    try {
        // Create helpers
        var helpers = CreateCodecHelpers();

        // Parse and validate codec script
        if (!codecScript || codecScript.trim() === '') {
            throw new Error("Codec script is empty");
        }

        // Build the function call with proper arguments
        var functionCall;
        if (functionName === 'OnUplink') {
            functionCall = 'return OnUplink();';
        } else if (functionName === 'OnDownlink') {
            functionCall = 'return OnDownlink(__arg0__, __arg1__);';
        } else {
            functionCall = 'return ' + functionName + '();';
        }

        // Create isolated execution context
        // Inject helpers into scope, block access to window/document/global
        var executeCode = new Function(
            'getState', 'setState',
            'getSendInterval', 'setSendInterval',
            'hexToBytes', 'base64ToBytes',
            'log',
            '__arg0__', '__arg1__',
            `
            "use strict";

            // Block access to global objects
            var window = undefined;
            var document = undefined;
            var global = undefined;
            var self = undefined;

            // User codec script
            ${codecScript}

            // Execute requested function
            if (typeof ${functionName} !== 'function') {
                throw new Error('Function ${functionName} is not defined in codec');
            }

            ${functionCall}
            `
        );

        // Execute with helper functions and actual arguments
        result = executeCode.call(
            null,
            helpers.getState,
            helpers.setState,
            helpers.getSendInterval,
            helpers.setSendInterval,
            helpers.hexToBytes,
            helpers.base64ToBytes,
            helpers.log,
            args[0],  // bytes for OnDownlink
            args[1]   // fPort for OnDownlink
        );

    } catch (e) {
        error = e;
        CodecTestState.addLog('ERROR: ' + e.message, 'error');
    }

    var endTime = performance.now();
    var executionTimeMs = (endTime - startTime).toFixed(2);

    return {
        result: result,
        error: error,
        executionTimeMs: executionTimeMs
    };
}

/**
 * Convert byte array to hex string with spaces
 */
function BytesToHexString(bytes) {
    if (!Array.isArray(bytes)) {
        return '';
    }
    var result = [];
    for (var i = 0; i < bytes.length; i++) {
        var byte = bytes[i];
        if (byte === undefined || byte === null) {
            byte = 0;
        }
        result.push(('0' + (byte & 0xFF).toString(16).toUpperCase()).slice(-2));
    }
    return result.join(' ');
}

/**
 * Parse hex string with spaces to byte array
 */
function ParseHexInput(hexString) {
    if (!hexString || hexString.trim() === '') {
        return [];
    }

    // Remove extra spaces and split
    var hexParts = hexString.trim().split(/\s+/);
    var bytes = [];

    for (var i = 0; i < hexParts.length; i++) {
        var hex = hexParts[i];
        if (!/^[0-9A-Fa-f]{1,2}$/.test(hex)) {
            throw new Error('Invalid hex byte: ' + hex);
        }
        bytes.push(parseInt(hex, 16));
    }

    return bytes;
}

/**
 * Copy text to clipboard
 */
function CopyToClipboard(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
        // Modern Clipboard API
        navigator.clipboard.writeText(text).then(function() {
            Show_SweetToast("Copied to clipboard", "");
        }).catch(function(err) {
            console.error('Failed to copy:', err);
            Show_ErrorSweetToast("Copy failed", "Please copy manually");
        });
    } else {
        // Fallback: create temporary textarea
        var textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.style.position = 'fixed';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.select();

        try {
            document.execCommand('copy');
            Show_SweetToast("Copied to clipboard", "");
        } catch (err) {
            console.error('Failed to copy:', err);
            Show_ErrorSweetToast("Copy failed", "Please copy manually");
        }

        document.body.removeChild(textarea);
    }
}

/**
 * Test OnUplink button click handler
 */
$("#btn-test-uplink").on('click', function() {
    var codecScript = GetCodecEditorValue();

    if (!codecScript || codecScript.trim() === '') {
        Show_ErrorSweetToast("Error", "Codec script is empty");
        return;
    }

    // Clear previous output
    $("#output-uplink-bytes").val('');
    $("#output-uplink-fport").val('');
    $("#uplink-exec-time").hide();

    CodecTestState.addLog('Executing OnUplink()...', 'info');

    // Execute OnUplink
    var execution = ExecuteCodecFunction(codecScript, 'OnUplink', []);

    // Show execution time
    $("#uplink-exec-time").text('Executed in ' + execution.executionTimeMs + 'ms').show();

    if (execution.error) {
        Show_ErrorSweetToast("Execution Error", execution.error.message);
        return;
    }

    // Process result
    var result = execution.result;
    var bytes = null;
    var fPort = null;

    // OnUplink can return:
    // 1. Byte array: [0x01, 0x02, 0x03]
    // 2. Object: { fPort: 85, bytes: [0x01, 0x02] }
    if (Array.isArray(result)) {
        bytes = result;
        fPort = 'default';
    } else if (result && typeof result === 'object') {
        bytes = result.bytes || [];
        fPort = result.fPort !== undefined ? result.fPort : 'default';
    } else {
        Show_ErrorSweetToast("Invalid Return", "OnUplink must return byte array or {fPort, bytes}");
        return;
    }

    // Display output
    var hexString = BytesToHexString(bytes);
    $("#output-uplink-bytes").val(hexString);
    $("#output-uplink-fport").val(fPort);

    CodecTestState.addLog('OnUplink() completed: ' + bytes.length + ' bytes', 'info');
});

/**
 * Test OnDownlink button click handler
 */
$("#btn-test-downlink").on('click', function() {
    var codecScript = GetCodecEditorValue();

    if (!codecScript || codecScript.trim() === '') {
        Show_ErrorSweetToast("Error", "Codec script is empty");
        return;
    }

    // Parse input
    var hexInput = $("#input-downlink-bytes").val();
    var fPort = parseInt($("#input-downlink-fport").val());

    if (!hexInput || hexInput.trim() === '') {
        Show_ErrorSweetToast("Error", "Input bytes are required");
        return;
    }

    if (isNaN(fPort) || fPort < 1 || fPort > 223) {
        Show_ErrorSweetToast("Error", "fPort must be between 1 and 223");
        return;
    }

    // Parse hex bytes
    var bytes = null;
    try {
        bytes = ParseHexInput(hexInput);
    } catch (e) {
        Show_ErrorSweetToast("Parse Error", e.message);
        return;
    }

    // Clear previous output
    $("#downlink-exec-time").hide();

    CodecTestState.addLog('Executing OnDownlink([' + bytes.join(', ') + '], ' + fPort + ')...', 'info');

    // Execute OnDownlink
    var execution = ExecuteCodecFunction(codecScript, 'OnDownlink', [bytes, fPort]);

    // Show execution time
    $("#downlink-exec-time").text('Executed in ' + execution.executionTimeMs + 'ms').show();

    if (execution.error) {
        // OnDownlink is optional, so function not found is acceptable
        if (execution.error.message.includes('not defined')) {
            Show_ErrorSweetToast("Function Not Found", "OnDownlink is not defined in this codec (optional)");
        } else {
            // Provide detailed error info
            var errorMsg = execution.error.message;
            var errorDetails = "Error in OnDownlink: " + errorMsg;

            // Add stack trace if available
            if (execution.error.stack) {
                errorDetails += "\n\nStack: " + execution.error.stack;
            }

            Show_ErrorSweetToast("Execution Error", errorMsg);
            CodecTestState.addLog('Full error: ' + errorDetails, 'error');
        }
    } else {
        CodecTestState.addLog('OnDownlink() completed', 'info');
    }
});

/**
 * Copy uplink bytes button
 */
$("#btn-copy-uplink-bytes").on('click', function() {
    var bytes = $("#output-uplink-bytes").val();
    if (bytes) {
        CopyToClipboard(bytes);
    }
});

/**
 * Clear logs button
 */
$("#btn-clear-logs").on('click', function() {
    CodecTestState.clearLogs();
});

/**
 * Reset state button
 */
$("#btn-reset-state").on('click', function() {
    swal({
        title: "Clear Test State?",
        text: "This will reset all state variables and logs.",
        icon: "warning",
        buttons: true,
    }).then((willReset) => {
        if (willReset) {
            CodecTestState.reset();
            Show_SweetToast("State Cleared", "");
        }
    });
});

/**
 * Initialize test panel when codec form is loaded
 */
function InitializeCodecTestPanel() {
    // Reset test state when switching codecs
    CodecTestState.reset();
}

// Hook into existing LoadCodec (view mode)
var originalLoadCodec = LoadCodec;
LoadCodec = function(codec) {
    originalLoadCodec(codec);
    InitializeCodecTestPanel();
    // Hide test panel in view mode
    $("#codec-test-panel").hide();
};

// Hook into existing CleanCodecForm (add/edit mode)
var originalCleanCodecForm = CleanCodecForm;
CleanCodecForm = function() {
    originalCleanCodecForm();
    InitializeCodecTestPanel();
    // Show test panel in add/edit mode
    $("#codec-test-panel").show();
};

// Show test panel when edit button is clicked
$("[name=btn-edit-codec]").on('click', function() {
    $("#codec-test-panel").show();
});

/**
 * Auto-format hex input field
 * Automatically adds spaces and capitalizes hex characters
 */
$("#input-downlink-bytes").on('input', function() {
    var input = $(this).val();

    // Remove all non-hex characters and spaces
    var cleaned = input.replace(/[^0-9A-Fa-f]/g, '');

    // Capitalize
    cleaned = cleaned.toUpperCase();

    // Add space every 2 characters
    var formatted = '';
    for (var i = 0; i < cleaned.length; i++) {
        if (i > 0 && i % 2 === 0) {
            formatted += ' ';
        }
        formatted += cleaned[i];
    }

    // Update input if changed
    if (input !== formatted) {
        var cursorPos = this.selectionStart;
        var oldLength = input.length;
        $(this).val(formatted);

        // Adjust cursor position
        var newLength = formatted.length;
        var newCursorPos = cursorPos + (newLength - oldLength);
        this.setSelectionRange(newCursorPos, newCursorPos);
    }
});
