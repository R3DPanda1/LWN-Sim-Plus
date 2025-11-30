/**
 * Codec Management JavaScript
 * Handles UI for creating, editing, and deleting custom JavaScript codecs
 */

"use strict";

var Codecs = new Map();

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
    var createdDate = new Date(codec.createdAt).toLocaleDateString();

    var item = "<tr data-id=\""+codec.id+"\">\
                    <td class=\"clickable text-orange font-weight-bold font-italic\" >"+codec.name+"</td>\
                    <td>"+codec.description+"</td> \
                    <td>"+codec.version+"</td>\
                    <td>"+createdDate+"</td>\
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
    $("[name=input-codec-description]").val(codec.description);
    $("[name=input-codec-version]").val(codec.version);
    $("[name=input-codec-author]").val(codec.author);
    $("#textarea-codec-script").val(codec.script);

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
    $("[name=input-codec-description]").val("");
    $("[name=input-codec-version]").val("1.0");
    $("[name=input-codec-author]").val("");
    $("#textarea-codec-script").val("");
    $("#textarea-codec-default-config").val("");

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
    $("[name=input-codec-description]").prop("disabled", false);
    $("[name=input-codec-version]").prop("disabled", false);
    $("[name=input-codec-author]").prop("disabled", false);
    $("#textarea-codec-script").prop("disabled", false);
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
    var description = $("[name=input-codec-description]").val();
    var version = $("[name=input-codec-version]").val();
    var author = $("[name=input-codec-author]").val();
    var script = $("#textarea-codec-script").val();
    var defaultConfig = $("#textarea-codec-default-config]").val();

    if(!name || !script){
        Show_ErrorSweetToast("Error", "Name and Script are required");
        return;
    }

    var codecData = {
        "name": name,
        "description": description,
        "version": version,
        "author": author,
        "script": script,
        "defaultPayloadConfig": defaultConfig
    };

    var jsonData = JSON.stringify(codecData);

    $.post(url + "/api/add-codec", jsonData, "json")
    .done((data)=>{
        Show_SweetToast("Codec saved successfully", "");
        CleanCodecForm();
        LoadCodecList();
        PopulateCodecDropdown();
        ShowList($("#codecs"),"List codecs",false);
    })
    .fail((data)=>{
        var errorMsg = data.responseJSON ? data.responseJSON.error : data.statusText;
        Show_ErrorSweetToast("Failed to save codec", errorMsg);
    });
}

// Delete codec function
function DeleteCodec(codecId){
    var jsonData = JSON.stringify({"id": codecId});

    $.post(url + "/api/delete-codec", jsonData, "json")
    .done((data)=>{
        Show_SweetToast("Codec deleted successfully", "");
        CleanCodecForm();
        LoadCodecList();
        PopulateCodecDropdown();
        ShowList($("#codecs"),"List codecs",false);
    })
    .fail((data)=>{
        var errorMsg = data.responseJSON ? data.responseJSON.error : data.statusText;
        Show_ErrorSweetToast("Failed to delete codec", errorMsg);
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
