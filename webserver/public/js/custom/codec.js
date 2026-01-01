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
    var id = $(this).data("id");
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
    var isEdit = codecId && codecId !== "";
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
