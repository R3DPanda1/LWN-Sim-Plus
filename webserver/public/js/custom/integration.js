/**
 * Integration Management JavaScript
 * Handles UI for creating, editing, and deleting ChirpStack integrations
 */

"use strict";

var Integrations = new Map();

// Load integrations from API and populate the list
function LoadIntegrationList() {
    $.ajax({
        url: url+"/api/integrations",
        type:"GET",
        headers:{
            "Access-Control-Allow-Origin":"*"
        }
    }).done((data)=>{
        $("#list-integrations").empty();
        Integrations.clear();

        if(data.integrations && data.integrations.length > 0){
            data.integrations.forEach(integration => {
                Integrations.set(integration.id, integration);
                AddItemListIntegrations(integration);
            });
        }

        // Also update device integration dropdown
        PopulateDeviceIntegrationDropdown();
    }).fail((data)=>{
        console.error("Unable to load integrations", data.statusText);
    });
}

// Add integration item to the list
function AddItemListIntegrations(integration){
    var statusImg = integration.enabled ?
        './img/green_circle.svg' :
        './img/red_circle.svg';

    var item = "<tr data-id=\""+integration.id+"\">\
                    <th scope=\"row\"> \
                        <img src=\""+statusImg+"\">\
                    </th>\
                    <td class=\"clickable font-weight-bold font-italic text-navy\" >"+integration.name+"</td>\
                    <td>"+integration.type+"</td>\
                    <td>"+integration.url+"</td>\
                </tr>";

    $("#list-integrations").append(item);
}

// Handle integration list item click - load integration for editing
$(document).on('click', '#list-integrations tr', function(){
    var id = $(this).data("id");
    var integrationMeta = Integrations.get(id);

    if(!integrationMeta){
        return;
    }

    // Fetch full integration details
    $.ajax({
        url: url+"/api/integration/"+id,
        type:"GET",
        headers:{
            "Access-Control-Allow-Origin":"*"
        }
    }).done((data)=>{
        if(data.integration){
            LoadIntegration(data.integration);
            ShowList($("#add-integration"),"Edit Integration",false);
        }
    }).fail((data)=>{
        Show_ErrorSweetToast("Unable to load integration details", data.statusText);
    });
});

// Load integration data into the form
function LoadIntegration(integration){
    $("[name=input-integration-name]").val(integration.name);
    $("#select-integration-type").val(integration.type);
    $("[name=input-integration-url]").val(integration.url);
    $("[name=input-integration-apikey]").val(integration.apiKey);
    $("[name=input-integration-tenantid]").val(integration.tenantId);
    $("[name=input-integration-appid]").val(integration.applicationId);
    $("#checkbox-integration-enabled").prop("checked", integration.enabled);

    // Set all fields to disabled (view-only mode)
    $("[name=input-integration-name]").prop("disabled", true);
    $("#select-integration-type").prop("disabled", true);
    $("[name=input-integration-url]").prop("disabled", true);
    $("[name=input-integration-apikey]").prop("disabled", true);
    $("[name=input-integration-tenantid]").prop("disabled", true);
    $("[name=input-integration-appid]").prop("disabled", true);
    $("#checkbox-integration-enabled").prop("disabled", true);

    $("#div-buttons-integration").data("id", integration.id);
    $("[name=btn-test-integration]").show();
    $("[name=btn-delete-integration]").show();
    $("[name=btn-edit-integration]").show();
    $("[name=btn-save-integration]").hide();

    $("#integrations").removeClass("show active");
    $("#add-integration").addClass("show active");
    $(".section-header h1").text("Edit Integration");
}

// Clean integration form
function CleanIntegrationForm(){
    $("[name=input-integration-name]").val("");
    $("#select-integration-type").val("chirpstack");
    $("[name=input-integration-url]").val("");
    $("[name=input-integration-apikey]").val("");
    $("[name=input-integration-tenantid]").val("");
    $("[name=input-integration-appid]").val("");
    $("#checkbox-integration-enabled").prop("checked", true);

    // Enable all fields for new integration
    $("[name=input-integration-name]").prop("disabled", false);
    $("#select-integration-type").prop("disabled", false);
    $("[name=input-integration-url]").prop("disabled", false);
    $("[name=input-integration-apikey]").prop("disabled", false);
    $("[name=input-integration-tenantid]").prop("disabled", false);
    $("[name=input-integration-appid]").prop("disabled", false);
    $("#checkbox-integration-enabled").prop("disabled", false);

    $("#div-buttons-integration").removeData("id");
    $("[name=btn-test-integration]").show();
    $("[name=btn-delete-integration]").hide();
    $("[name=btn-edit-integration]").hide();
    $("[name=btn-save-integration]").show();
}

// Save integration button click
$("[name=btn-save-integration]").on('click', function(){
    SaveIntegration();
});

// Edit integration button click
$("[name=btn-edit-integration]").on('click', function(){
    $(this).hide();
    $("[name=btn-test-integration]").show();
    $("[name=btn-delete-integration]").hide();
    $("[name=btn-save-integration]").show();

    $("[name=input-integration-name]").prop("disabled", false);
    $("#select-integration-type").prop("disabled", false);
    $("[name=input-integration-url]").prop("disabled", false);
    $("[name=input-integration-apikey]").prop("disabled", false);
    $("[name=input-integration-tenantid]").prop("disabled", false);
    $("[name=input-integration-appid]").prop("disabled", false);
    $("#checkbox-integration-enabled").prop("disabled", false);
});

// Delete integration button click
$("[name=btn-delete-integration]").on('click', function(){
    var integrationId = $("#div-buttons-integration").data("id");

    if(!integrationId){
        Show_ErrorSweetToast("Error", "No integration selected");
        return;
    }

    swal({
        title: "Are you sure?",
        text: "Once deleted, devices using this integration will no longer be provisioned to ChirpStack!",
        icon: "warning",
        buttons: true,
        dangerMode: true,
    })
    .then((willDelete) => {
        if (willDelete) {
            DeleteIntegration(integrationId);
        }
    });
});

// Test connection button click
$("[name=btn-test-integration]").on('click', function(){
    var integrationId = $("#div-buttons-integration").data("id");

    if(!integrationId){
        // For new integration, test with form values
        TestIntegrationFromForm();
    } else {
        // For existing integration, test saved config
        TestIntegrationConnection(integrationId);
    }
});

// Test integration connection
function TestIntegrationConnection(integrationId) {
    $.ajax({
        url: url + "/api/integration/" + integrationId + "/test",
        type: "POST",
        headers: {
            "Access-Control-Allow-Origin": "*"
        }
    }).done((data) => {
        Show_SweetToast("Connection successful", "ChirpStack server is reachable");
    }).fail((data) => {
        var errorMsg = data.responseJSON ? data.responseJSON.error : data.statusText;
        Show_ErrorSweetToast("Connection failed", errorMsg);
    });
}

// Test integration from form values (for new integrations)
function TestIntegrationFromForm() {
    var urlVal = $("[name=input-integration-url]").val();
    var apiKey = $("[name=input-integration-apikey]").val();

    if(!urlVal || !apiKey){
        Show_ErrorSweetToast("Error", "URL and API Key are required to test connection");
        return;
    }

    Show_SweetToast("Testing...", "Please wait while we test the connection");

    // For new integration, we need to save it first, test, then delete if user doesn't save
    // For simplicity, just inform user to save first
    Show_ErrorSweetToast("Save Required", "Please save the integration first to test the connection");
}

// Save integration function
function SaveIntegration(){
    var name = $("[name=input-integration-name]").val();
    var type = $("#select-integration-type").val();
    var urlVal = $("[name=input-integration-url]").val();
    var apiKey = $("[name=input-integration-apikey]").val();
    var tenantId = $("[name=input-integration-tenantid]").val();
    var appId = $("[name=input-integration-appid]").val();
    var enabled = $("#checkbox-integration-enabled").prop("checked");
    var integrationId = $("#div-buttons-integration").data("id");

    if(!name || !urlVal || !apiKey || !tenantId || !appId){
        Show_ErrorSweetToast("Error", "All fields are required");
        return;
    }

    // Determine if we're editing or adding
    var isEdit = integrationId && integrationId !== "";
    var apiEndpoint = isEdit ? "/api/update-integration" : "/api/add-integration";

    var integrationData = {
        "name": name,
        "type": type,
        "url": urlVal,
        "apiKey": apiKey,
        "tenantId": tenantId,
        "applicationId": appId,
        "enabled": enabled
    };

    // Add ID if editing
    if(isEdit){
        integrationData.id = integrationId;
    }

    var jsonData = JSON.stringify(integrationData);

    $.post(url + apiEndpoint, jsonData, "json")
    .done((data)=>{
        Show_SweetToast(isEdit ? "Integration updated successfully" : "Integration saved successfully", "");
        CleanIntegrationForm();
        LoadIntegrationList();
        ShowList($("#integrations"),"List integrations",false);
    })
    .fail((data)=>{
        var errorMsg = data.responseJSON ? data.responseJSON.error : data.statusText;
        Show_ErrorSweetToast(isEdit ? "Failed to update integration" : "Failed to save integration", errorMsg);
    });
}

// Delete integration function
function DeleteIntegration(integrationId){
    var jsonData = JSON.stringify({"id": integrationId});
    $.post(url + "/api/delete-integration", jsonData, "json")
    .done((data)=>{
        Show_SweetToast("Integration deleted successfully", "");
        CleanIntegrationForm();
        LoadIntegrationList();
        ShowList($("#integrations"),"List integrations",false);
    })
    .fail((data)=>{
        var errorMsg = data.responseJSON ? data.responseJSON.error : data.statusText;
        Show_ErrorSweetToast("Failed to delete integration", errorMsg);
    });
}

// Populate device integration dropdown
function PopulateDeviceIntegrationDropdown() {
    var dropdown = $("#select-dev-integration");
    dropdown.empty();
    dropdown.append('<option value="">Select an integration...</option>');

    Integrations.forEach((integration, id) => {
        if(integration.enabled) {
            dropdown.append('<option value="' + id + '">' + integration.name + '</option>');
        }
    });
}

// Load device profiles from ChirpStack
function LoadDeviceProfiles(integrationId) {
    var dropdown = $("#select-dev-profile");
    dropdown.empty();
    dropdown.append('<option value="">Loading device profiles...</option>');

    if(!integrationId) {
        dropdown.empty();
        dropdown.append('<option value="">Select an integration first...</option>');
        return;
    }

    $.ajax({
        url: url + "/api/integration/" + integrationId + "/device-profiles",
        type: "GET",
        headers: {
            "Access-Control-Allow-Origin": "*"
        }
    }).done((data) => {
        dropdown.empty();
        dropdown.append('<option value="">Select a device profile...</option>');

        if(data.deviceProfiles && data.deviceProfiles.length > 0) {
            data.deviceProfiles.forEach(profile => {
                dropdown.append('<option value="' + profile.id + '">' + profile.name + '</option>');
            });
        }
    }).fail((data) => {
        dropdown.empty();
        dropdown.append('<option value="">Failed to load profiles</option>');
        console.error("Unable to load device profiles", data.statusText);
    });
}

// Toggle device integration settings visibility
$("#checkbox-dev-integration-enabled").on('change', function(){
    if($(this).prop("checked")) {
        $("#device-integration-settings").removeClass("hide");
        LoadIntegrationList(); // Refresh integration list

        // Auto-generate AppKey if empty and OTAA is enabled
        var appKey = $("#appkey").val();
        if(!appKey || appKey === "") {
            var generatedKey = GenerateRandomKey();
            $("#appkey").val(generatedKey);
        }
    } else {
        $("#device-integration-settings").addClass("hide");
    }
});

// When integration is selected, load device profiles
$("#select-dev-integration").on('change', function(){
    var integrationId = $(this).val();
    LoadDeviceProfiles(integrationId);
});

// Generate random 128-bit key (32 hex chars)
function GenerateRandomKey() {
    var hex = '';
    for (var i = 0; i < 32; i++) {
        hex += Math.floor(Math.random() * 16).toString(16);
    }
    return hex;
}

// API key visibility toggle for integration form
$("[name=btn-watch-apikey]").on('click', function(){
    var input = $("[name=input-integration-apikey]");
    if(input.attr("type") === "password"){
        input.attr("type", "text");
    } else {
        input.attr("type", "password");
    }
});

// Initialize integration list on page load
$("#integrations-tab").on('click', function(){
    LoadIntegrationList();
    $(".section-header h1").text("List Integrations");
});

// Clean form when adding new integration
$("#add-integration-tab").on('click', function(){
    CleanIntegrationForm();
    $(".section-header h1").text("Add New Integration");
});

// Socket event handlers for real-time updates
$(document).ready(function(){
    // Load integrations on startup (for device form dropdown)
    LoadIntegrationList();
});
