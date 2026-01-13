"use strict";

// Template management module
var Templates = new Map();
var MapBulkCreate;
var MarkerBulkCreate;

// Region mapping for display
var RegionNames = {
    1: "EU868",
    2: "US915",
    3: "CN779",
    4: "EU433",
    5: "AU915",
    6: "CN470",
    7: "AS923",
    8: "KR920",
    9: "IN865",
    10: "RU864"
};

// ==================== Template List ====================

function LoadTemplateList() {
    $.ajax({
        url: url + "/api/templates",
        type: "GET"
    }).done(function(data) {
        Templates.clear();
        $("#list-templates").empty();

        if (data.templates && data.templates.length > 0) {
            data.templates.forEach(function(template) {
                Templates.set(template.id, template);
                AddItemListTemplates(template);
            });
        }

        // Also populate bulk create dropdown
        PopulateBulkTemplateDropdown();
    }).fail(function(err) {
        console.error("Failed to load templates:", err);
    });
}

function AddItemListTemplates(template) {
    var codecName = template.useCodec && template.codecId ? GetCodecName(template.codecId) : "Static Payload";
    var regionName = RegionNames[template.region] || "Unknown";

    var row = '<tr class="click-template" data-id="' + template.id + '">' +
        '<td class="clickable font-weight-bold font-italic text-navy">' + template.name + '</td>' +
        '<td>' + regionName + '</td>' +
        '<td>' + codecName + '</td>' +
        '</tr>';

    $("#list-templates").append(row);
}

function GetCodecName(codecId) {
    // Try to get codec name from the codec dropdown options
    var codecOption = $("#select-template-codec option[value='" + codecId + "']");
    if (codecOption.length > 0) {
        return codecOption.text();
    }
    return codecId;
}

// ==================== Template Form ====================

function CleanTemplateForm() {
    $("[name=input-template-name]").val("");
    $("#select-template-region").val("1");
    $("#checkbox-template-classb").prop("checked", false);
    $("#checkbox-template-classc").prop("checked", false);
    $("#checkbox-template-adr").prop("checked", true);
    $("[name=input-template-range]").val("10000");
    $("[name=input-template-datarate]").val("0");
    $("[name=input-template-sendinterval]").val("60");
    $("[name=input-template-rx1duration]").val("3000");
    $("[name=input-template-rx2duration]").val("3000");
    $("[name=input-template-fport]").val("1");
    $("#select-template-codec").val("");
    $("#checkbox-template-integration-enabled").prop("checked", false);
    $("#template-integration-settings").addClass("hide");
    $("#select-template-integration").val("");
    $("#select-template-device-profile").val("");

    // Reset buttons
    $("#div-buttons-template").attr("data-id", "");
    $("[name=btn-delete-template]").addClass("hide");
    $("[name=btn-edit-template]").addClass("hide");
    $("[name=btn-save-template]").removeClass("hide");

    // Enable all inputs
    SetTemplateFormEnabled(true);
}

function SetTemplateFormEnabled(enabled) {
    $("[name=input-template-name]").prop("disabled", !enabled);
    $("#select-template-region").prop("disabled", !enabled);
    $("#checkbox-template-classb").prop("disabled", !enabled);
    $("#checkbox-template-classc").prop("disabled", !enabled);
    $("#checkbox-template-adr").prop("disabled", !enabled);
    $("[name=input-template-range]").prop("disabled", !enabled);
    $("[name=input-template-datarate]").prop("disabled", !enabled);
    $("[name=input-template-sendinterval]").prop("disabled", !enabled);
    $("[name=input-template-rx1duration]").prop("disabled", !enabled);
    $("[name=input-template-rx2duration]").prop("disabled", !enabled);
    $("[name=input-template-fport]").prop("disabled", !enabled);
    $("#select-template-codec").prop("disabled", !enabled);
    $("#checkbox-template-integration-enabled").prop("disabled", !enabled);
    $("#select-template-integration").prop("disabled", !enabled);
    $("#select-template-device-profile").prop("disabled", !enabled);
}

function LoadTemplate(template) {
    $("[name=input-template-name]").val(template.name);
    $("#select-template-region").val(template.region);
    $("#checkbox-template-classb").prop("checked", template.supportedClassB);
    $("#checkbox-template-classc").prop("checked", template.supportedClassC);
    $("#checkbox-template-adr").prop("checked", template.supportedADR);
    $("[name=input-template-range]").val(template.range);
    $("[name=input-template-datarate]").val(template.dataRate);
    $("[name=input-template-sendinterval]").val(template.sendInterval);
    $("[name=input-template-rx1duration]").val(template.rx1Duration || 3000);
    $("[name=input-template-rx2duration]").val(template.rx2Duration || 3000);
    $("[name=input-template-fport]").val(template.fport);
    $("#select-template-codec").val(template.codecId || "");

    if (template.integrationEnabled) {
        $("#checkbox-template-integration-enabled").prop("checked", true);
        $("#template-integration-settings").removeClass("hide");
        LoadTemplateIntegrationList();
        setTimeout(function() {
            // Handle ID 0 properly - only use empty string if integrationId is null/undefined
            var integrationIdVal = (template.integrationId !== undefined && template.integrationId !== null) ? template.integrationId : "";
            $("#select-template-integration").val(integrationIdVal);
            if (template.integrationId !== undefined && template.integrationId !== null) {
                // Pass saved profile ID for offline fallback
                LoadTemplateDeviceProfiles(template.integrationId, template.deviceProfileId);
            }
        }, 200);
    } else {
        $("#checkbox-template-integration-enabled").prop("checked", false);
        $("#template-integration-settings").addClass("hide");
    }

    // Set buttons
    $("#div-buttons-template").attr("data-id", template.id);
    $("[name=btn-delete-template]").removeClass("hide");
    $("[name=btn-edit-template]").removeClass("hide");
    $("[name=btn-save-template]").addClass("hide");

    // Disable form for viewing
    SetTemplateFormEnabled(false);
}

function SaveTemplate() {
    var templateId = $("#div-buttons-template").attr("data-id");
    var isUpdate = templateId && templateId !== "";

    var template = {
        id: parseInt(templateId) || 0,
        name: $("[name=input-template-name]").val(),
        region: parseInt($("#select-template-region").val()),
        supportedClassB: $("#checkbox-template-classb").prop("checked"),
        supportedClassC: $("#checkbox-template-classc").prop("checked"),
        supportedADR: $("#checkbox-template-adr").prop("checked"),
        range: parseFloat($("[name=input-template-range]").val()),
        dataRate: parseInt($("[name=input-template-datarate]").val()),
        sendInterval: parseInt($("[name=input-template-sendinterval]").val()),
        fport: parseInt($("[name=input-template-fport]").val()),
        useCodec: $("#select-template-codec").val() !== "",
        codecId: parseInt($("#select-template-codec").val()) || 0,
        integrationEnabled: $("#checkbox-template-integration-enabled").prop("checked"),
        integrationId: parseInt($("#select-template-integration").val()) || 0,
        deviceProfileId: $("#select-template-device-profile").val() || "",
        // RX window settings (read from form)
        rx1Delay: 1000,
        rx1Duration: parseInt($("[name=input-template-rx1duration]").val()) || 3000,
        rx1DROffset: 0,
        rx2Delay: 2000,
        rx2Duration: parseInt($("[name=input-template-rx2duration]").val()) || 3000,
        rx2Frequency: 869525000,
        rx2DataRate: 0,
        ackTimeout: 2,
        nbRetransmission: 1,
        mtype: 0,
        supportedFragment: false
    };

    // Validate
    if (!template.name) {
        Show_ErrorSweetToast("Error", "Template name is required");
        return;
    }

    var endpoint = isUpdate ? "/api/update-template" : "/api/add-template";

    $.ajax({
        url: url + endpoint,
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify(template)
    }).done(function(result) {
        if (result.error) {
            Show_ErrorSweetToast("Error", result.error);
        } else {
            Show_iziToast("Template saved successfully", "");
            LoadTemplateList();
            // Switch to template list tab
            $('[data-tab="#templates"]').click();
        }
    }).fail(function(err) {
        Show_ErrorSweetToast("Error", err.responseJSON ? err.responseJSON.error : "Failed to save template");
    });
}

function DeleteTemplate(templateId) {
    if (!confirm("Are you sure you want to delete this template?")) {
        return;
    }

    $.ajax({
        url: url + "/api/delete-template",
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({ id: parseInt(templateId) })
    }).done(function(result) {
        if (result.error) {
            Show_ErrorSweetToast("Error", result.error);
        } else {
            Show_iziToast("Template deleted successfully", "");
            LoadTemplateList();
            CleanTemplateForm();
            $('[data-tab="#templates"]').click();
        }
    }).fail(function(err) {
        Show_ErrorSweetToast("Error", err.responseJSON ? err.responseJSON.error : "Failed to delete template");
    });
}

// ==================== Integration Support ====================

function LoadTemplateIntegrationList() {
    $.ajax({
        url: url + "/api/integrations",
        type: "GET"
    }).done(function(data) {
        var select = $("#select-template-integration");
        select.empty();
        select.append('<option value="">Select integration...</option>');

        if (data.integrations && data.integrations.length > 0) {
            data.integrations.forEach(function(integration) {
                if (integration.enabled) {
                    select.append('<option value="' + integration.id + '">' + integration.name + '</option>');
                }
            });
        }
    });
}

// Load device profiles for template form
// savedProfileId: optional - the currently saved device profile ID to preserve if API fails
function LoadTemplateDeviceProfiles(integrationId, savedProfileId) {
    var select = $("#select-template-device-profile");
    select.empty();
    select.append('<option value="">Loading device profiles...</option>');

    if (integrationId === undefined || integrationId === null || integrationId === "") {
        select.empty();
        select.append('<option value="">Select an integration first...</option>');
        return;
    }

    $.ajax({
        url: url + "/api/integration/" + integrationId + "/device-profiles",
        type: "GET"
    }).done(function(data) {
        select.empty();
        select.append('<option value="">Select device profile...</option>');

        if (data.deviceProfiles && data.deviceProfiles.length > 0) {
            data.deviceProfiles.forEach(function(profile) {
                // Show name with full ID
                var displayText = profile.name + ' (' + profile.id + ')';
                select.append('<option value="' + profile.id + '">' + displayText + '</option>');
            });
        }

        // Select the saved profile if provided
        if (savedProfileId) {
            select.val(savedProfileId);
        }
    }).fail(function(err) {
        select.empty();
        // If we have a saved profile ID, show it even if API failed
        if (savedProfileId) {
            select.append('<option value="">Select device profile...</option>');
            select.append('<option value="' + savedProfileId + '">' + savedProfileId + '</option>');
            select.val(savedProfileId);
        } else {
            select.append('<option value="">Failed to load profiles</option>');
        }
        console.error("Failed to load device profiles:", err);
    });
}

// ==================== Bulk Creation ====================

function PopulateBulkTemplateDropdown() {
    var select = $("#select-bulk-template");
    select.empty();
    select.append('<option value="">Select a template...</option>');

    // Templates is a Map, so iterate correctly
    if (Templates.size > 0) {
        Templates.forEach(function(template, id) {
            select.append('<option value="' + id + '">' + template.name + '</option>');
        });
    }
}

function InitBulkCreateMap() {
    if (MapBulkCreate) {
        // Map already initialized, just invalidate size
        setTimeout(function() {
            MapBulkCreate.invalidateSize();
        }, 200);
        return;
    }

    var osmUrl = 'http://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
    var osmAttrib = '&copy; <a href="http://osm.org/copyright">OpenStreetMap</a> contributors';
    var osm = new L.TileLayer(osmUrl, { attribution: osmAttrib });

    MapBulkCreate = new L.Map('map-bulk-create').addLayer(osm).setView([0, 0], 2);

    var icon = L.icon({
        iconUrl: './img/blue_marker.svg',
        iconSize: [32, 41],
        iconAnchor: [19, 41],
        popupAnchor: [1, -34],
        tooltipAnchor: [16, -28]
    });

    MarkerBulkCreate = L.marker([0, 0], { icon: icon }).addTo(MapBulkCreate);

    // Click to set location
    MapBulkCreate.on('click', function(e) {
        MarkerBulkCreate.setLatLng(e.latlng);
        $("#input-bulk-lat").val(e.latlng.lat.toFixed(6));
        $("#input-bulk-lng").val(e.latlng.lng.toFixed(6));
    });
}

function SubmitBulkCreate() {
    var templateId = $("#select-bulk-template").val();
    var count = parseInt($("#input-bulk-count").val());
    var namePrefix = $("#input-bulk-prefix").val();
    var baseLat = parseFloat($("#input-bulk-lat").val());
    var baseLng = parseFloat($("#input-bulk-lng").val());
    var spreadMeters = parseFloat($("#input-bulk-spread").val());

    // Validate
    if (!templateId) {
        Show_ErrorSweetToast("Error", "Please select a template");
        return;
    }
    if (!namePrefix) {
        Show_ErrorSweetToast("Error", "Please enter a name prefix");
        return;
    }
    if (count < 1 || count > 1000) {
        Show_ErrorSweetToast("Error", "Count must be between 1 and 1000");
        return;
    }

    var data = {
        templateId: parseInt(templateId),
        count: count,
        namePrefix: namePrefix,
        baseLat: baseLat,
        baseLng: baseLng,
        baseAlt: 0,
        spreadMeters: spreadMeters
    };

    // Show loading
    $("#btn-bulk-create-submit").prop("disabled", true).text("Creating...");

    $.ajax({
        url: url + "/api/create-devices-from-template",
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify(data)
    }).done(function(result) {
        if (result.error) {
            Show_ErrorSweetToast("Error", result.error);
        } else {
            Show_SweetToast("Success", result.created + " devices created");
            // Reload all data and switch to devices list
            Init();
            // Switch to devices list tab
            ShowList($("#devs"), "List devices", false);
        }
    }).fail(function(err) {
        Show_ErrorSweetToast("Error", err.responseJSON ? err.responseJSON.error : "Failed to create devices");
    }).always(function() {
        $("#btn-bulk-create-submit").prop("disabled", false).text("Create Devices");
    });
}

// ==================== Codec Dropdown ====================

function PopulateTemplateCodecDropdown() {
    $.ajax({
        url: url + "/api/codecs",
        type: "GET"
    }).done(function(data) {
        var select = $("#select-template-codec");
        select.empty();
        select.append('<option value="">Static Payload</option>');

        if (data.codecs && data.codecs.length > 0) {
            data.codecs.forEach(function(codec) {
                select.append('<option value="' + codec.id + '">' + codec.name + '</option>');
            });
        }
    });
}

// ==================== Event Handlers ====================

$(document).ready(function() {
    // Tab click handlers
    $("#templates-tab").on('click', function() {
        LoadTemplateList();
        PopulateTemplateCodecDropdown();
        $(".section-header h1").text("List Templates");
    });

    $("#add-template-tab").on('click', function() {
        CleanTemplateForm();
        PopulateTemplateCodecDropdown();
        $(".section-header h1").text("Add New Template");
    });

    // Template list row click
    $(document).on('click', '.click-template', function() {
        var templateId = parseInt($(this).attr("data-id"));
        var template = Templates.get(templateId);
        if (template) {
            PopulateTemplateCodecDropdown();
            setTimeout(function() {
                LoadTemplate(template);
            }, 100);
            $('[data-tab="#add-template"]').click();
        }
    });

    // Save button
    $("[name=btn-save-template]").on('click', function() {
        SaveTemplate();
    });

    // Edit button
    $("[name=btn-edit-template]").on('click', function() {
        SetTemplateFormEnabled(true);
        $("[name=btn-edit-template]").addClass("hide");
        $("[name=btn-save-template]").removeClass("hide");
    });

    // Delete button
    $("[name=btn-delete-template]").on('click', function() {
        var templateId = $("#div-buttons-template").attr("data-id");
        if (templateId) {
            DeleteTemplate(templateId);
        }
    });

    // Integration checkbox
    $("#checkbox-template-integration-enabled").on('change', function() {
        if ($(this).prop("checked")) {
            $("#template-integration-settings").removeClass("hide");
            LoadTemplateIntegrationList();
        } else {
            $("#template-integration-settings").addClass("hide");
        }
    });

    // Integration select change
    $("#select-template-integration").on('change', function() {
        var integrationId = $(this).val();
        LoadTemplateDeviceProfiles(integrationId);
    });

    // Bulk create tab - initialize when tab is shown
    $("#add-from-template-tab").on('click', function() {
        // Load templates first, then populate dropdown and initialize map
        $.ajax({
            url: url + "/api/templates",
            type: "GET"
        }).done(function(data) {
            Templates.clear();
            if (data.templates && data.templates.length > 0) {
                data.templates.forEach(function(template) {
                    Templates.set(template.id, template);
                });
            }
            // Now populate the dropdown
            PopulateBulkTemplateDropdown();
        }).fail(function(err) {
            console.error("Failed to load templates:", err);
        });

        // Initialize map after tab is visible
        setTimeout(function() {
            InitBulkCreateMap();
        }, 300);
    });

    // Bulk create submit
    $("#btn-bulk-create-submit").on('click', function() {
        SubmitBulkCreate();
    });

    // Bulk create coordinate inputs
    $("#input-bulk-lat, #input-bulk-lng").on('change', function() {
        var lat = parseFloat($("#input-bulk-lat").val()) || 0;
        var lng = parseFloat($("#input-bulk-lng").val()) || 0;
        if (MarkerBulkCreate && MapBulkCreate) {
            MarkerBulkCreate.setLatLng([lat, lng]);
            MapBulkCreate.setView([lat, lng], 10);
        }
    });
});
