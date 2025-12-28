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
            data.codecs.forEach(codec => {
                Codecs.set(codec.id, codec);
                AddItemListCodecs(codec);
            });
        }
    }).fail((data)=>{
        console.error("Unable to load codecs", data.statusText);
    });
}

// Add codec item to the list
function AddItemListCodecs(codec){
    var item = "<tr data-id=\""+codec.id+"\">\
                    <td class=\"clickable text-orange font-weight-bold font-italic\" >"+codec.name+"</td>\
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
    var skeletonCode = `// Encode function: Converts JSON object to byte array
// This function is called when the device sends an uplink
function Encode(fPort, obj) {
    var bytes = [];

    // Example: Encode temperature and humidity
    // Temperature: 2 bytes (signed int16, resolution 0.1°C)
    var temp = Math.round((obj.temperature || 20) * 10);
    bytes.push((temp >> 8) & 0xFF);
    bytes.push(temp & 0xFF);

    // Humidity: 1 byte (unsigned int, 0-100%)
    var humidity = Math.round(obj.humidity || 50);
    bytes.push(humidity & 0xFF);

    // Return payload with fPort and bytes
    return {
        fPort: fPort || 85,
        bytes: bytes
    };
}

// Decode function: Converts byte array to JSON object
// This function is used by the network server to decode the payload
function Decode(fPort, bytes) {
    var obj = {};

    // Example: Decode temperature and humidity
    if (bytes.length >= 3) {
        // Temperature: bytes 0-1 (signed int16, resolution 0.1°C)
        obj.temperature = (((bytes[0] << 8) | bytes[1]) << 16 >> 16) / 10.0;

        // Humidity: byte 2 (unsigned int, 0-100%)
        obj.humidity = bytes[2];
    }

    return obj;
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
            // Show warning with device count
            swal({
                title: "Cannot Delete Codec",
                text: "This codec is currently used by " + usageData.count + " device(s). Please remove the codec from all devices before deleting.",
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
