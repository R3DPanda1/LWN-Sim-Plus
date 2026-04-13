#!/usr/bin/env python3
"""
Seed script for k3d deployment.

Bootstraps ChirpStack and configures the simulator.
Idempotent: safe to run multiple times.

Requires:
  - kubectl configured with the k3d cluster context
  - ChirpStack and simulator accessible via NodePort (localhost:8090, localhost:8002)

Usage:
  python3 seed.py                          # uses defaults
  CS_URL=http://host:8090 python3 seed.py  # override URLs
"""

import requests
import subprocess
import time
import sys
import json
import os

CS_URL = os.environ.get("CS_URL", "http://localhost:8090")
SIM_URL = os.environ.get("SIM_URL", "http://localhost:8002/api")
NAMESPACE = os.environ.get("NAMESPACE", "lorawan")

AM319_CODEC_DECODE = """
function decodeUplink(input) {
    var bytes = input.bytes;
    var data = {};
    var i = 0;
    while (i < bytes.length) {
        var ch = bytes[i++];
        var type = bytes[i++];
        switch (type) {
            case 0x67:
                data.temperature = ((bytes[i] | (bytes[i+1] << 8)) << 16 >> 16) / 10;
                i += 2; break;
            case 0x68:
                data.humidity = bytes[i] / 2;
                i += 1; break;
            case 0x00:
                data.pir = bytes[i] === 1 ? "trigger" : "idle";
                i += 1; break;
            case 0xCB:
                data.light_level = bytes[i];
                i += 1; break;
            case 0x7D:
                var val = bytes[i] | (bytes[i+1] << 8);
                if (ch === 0x07) data.co2 = val;
                else if (ch === 0x08) data.tvoc = val / 100;
                else if (ch === 0x0A) data.hcho = val / 100;
                else if (ch === 0x0B) data.pm2_5 = val;
                else if (ch === 0x0C) data.pm10 = val;
                i += 2; break;
            case 0x73:
                data.pressure = (bytes[i] | (bytes[i+1] << 8)) / 10;
                i += 2; break;
            case 0xE6:
                data.tvoc = bytes[i] | (bytes[i+1] << 8);
                i += 2; break;
            default:
                i += 2;
        }
    }
    return { data: data };
}
"""

MCF_CODEC_DECODE = """
function decodeUplink(input) {
    var bytes = input.bytes;
    var data = {};
    if (bytes[0] === 0x01) {
        data.syncID = (bytes[1]|(bytes[2]<<8)|(bytes[3]<<16)|(bytes[4]<<24))>>>0;
        data.firmware = bytes[5]+"."+bytes[6]+"."+bytes[7];
        data.appType = bytes[8]|(bytes[9]<<8);
    } else if (bytes[0] === 0x0A) {
        var p = (bytes[1]|(bytes[2]<<8)|(bytes[3]<<16)|(bytes[4]<<24))>>>0;
        var sec=(p&0x1f)*2; p>>=5; var min=p&0x3f; p>>=6;
        var hr=p&0x1f; p>>=5; var dy=p&0x1f; p>>=5;
        var mo=p&0x0f; p>>=4; var yr=(p&0x7f)+2000;
        data.date = mo+"/"+dy+"/"+yr+", "+hr+":"+min+":"+sec;
        data.inputStatus8_1=bytes[5]; data.inputStatus9_16=bytes[6];
        data.inputStatus17_24=bytes[7]; data.inputStatus25_32=bytes[8];
        data.outputStatus8_1=bytes[9]; data.outputStatus9_16=bytes[10];
        data.outputStatus17_24=bytes[11]; data.outputStatus25_32=bytes[12];
        data.inputTrigger8_1=bytes[13]; data.inputTrigger9_16=bytes[14];
        data.inputTrigger17_24=bytes[15]; data.inputTrigger25_32=bytes[16];
    }
    return { data: data };
}
"""

SDM230_CODEC_DECODE = """
function decodeUplink(input) {
    var bytes = input.bytes;
    var data = {};
    function u32le(b,o){return (b[o]|(b[o+1]<<8)|(b[o+2]<<16)|(b[o+3]<<24))>>>0;}
    function f32be(b,o){
        var buf=new ArrayBuffer(4); var v=new DataView(buf);
        for(var i=0;i<4;i++) v.setUint8(i,b[o+i]);
        return v.getFloat32(0,false);
    }
    data.serialNumber = u32le(bytes,0);
    data.messageFragmentNumber = bytes[4];
    data.numberOfParameterBytes = bytes[5];
    data.E_M_ActiveEnergyTotal = Math.round(f32be(bytes,6)*100)/100;
    data.E_S_Voltage = Math.round(f32be(bytes,10)*100)/100;
    data.E_S_Current = Math.round(f32be(bytes,14)*10000)/10000;
    data.E_S_PowerFactor = Math.round(f32be(bytes,18)*10000)/10000;
    data.E_S_Frequency = Math.round(f32be(bytes,22)*100)/100;
    data.modbusChecksum = bytes[26]+","+bytes[27];
    return { data: data };
}
"""


def wait_for(name, check_fn, timeout=120):
    print(f"Waiting for {name}...")
    start = time.time()
    while time.time() - start < timeout:
        try:
            if check_fn():
                print(f"  {name} ready")
                return True
        except Exception:
            pass
        time.sleep(3)
    print(f"  Timed out waiting for {name}")
    return False


def create_api_key_via_kubectl():
    result = subprocess.run(
        ["kubectl", "exec", "-n", NAMESPACE, "deployment/chirpstack", "--",
         "chirpstack", "-c", "/etc/chirpstack", "create-api-key", "--name", "benchmark"],
        capture_output=True, text=True, timeout=30
    )
    output = result.stdout + result.stderr
    for line in output.split("\n"):
        if line.startswith("token:"):
            return line.split(":", 1)[1].strip()
    raise RuntimeError(f"Failed to create API key. Output:\n{output}")


def cs_headers(api_key):
    return {
        "Content-Type": "application/json",
        "Grpc-Metadata-Authorization": f"Bearer {api_key}"
    }


def cs_get(path, api_key):
    r = requests.get(f"{CS_URL}{path}", headers=cs_headers(api_key), timeout=10)
    r.raise_for_status()
    return r.json()


def cs_post(path, data, api_key):
    r = requests.post(f"{CS_URL}{path}", headers=cs_headers(api_key), json=data, timeout=10)
    if r.status_code >= 400:
        print(f"  CS POST {path}: {r.status_code} {r.text}")
    r.raise_for_status()
    return r.json()


def sim_get(path):
    r = requests.get(f"{SIM_URL}{path}", timeout=10)
    return r.json()


def sim_post(path, data):
    r = requests.post(f"{SIM_URL}{path}", json=data, timeout=10)
    return r.json()


def find_existing(items, name, key="name"):
    for item in items:
        if item.get(key) == name:
            return item
    return None


def main():
    # Wait for services
    if not wait_for("ChirpStack", lambda: requests.get(
            f"{CS_URL}/api/tenants?limit=1",
            headers={"Grpc-Metadata-Authorization": "Bearer test"},
            timeout=3).status_code in (200, 401)):
        sys.exit(1)

    if not wait_for("Simulator", lambda: requests.get(
            f"{SIM_URL}/status", timeout=3).status_code == 200):
        sys.exit(1)

    # Step 1: Create API key via kubectl exec
    print("\n--- Creating API key ---")
    api_key = create_api_key_via_kubectl()
    print(f"  Key: {api_key[:20]}...")

    # Step 2: Get tenant
    print("\n--- Getting tenant ---")
    tenants = cs_get("/api/tenants?limit=1", api_key)
    tenant_id = tenants["result"][0]["id"]
    print(f"  Tenant: {tenant_id}")

    # Step 3: Find or create application
    print("\n--- Application ---")
    apps = cs_get("/api/applications?limit=100&tenantId=" + tenant_id, api_key)
    existing_app = find_existing(apps.get("result", []), "Benchmark")
    if existing_app:
        app_id = existing_app["id"]
        print(f"  Found existing: {app_id}")
    else:
        resp = cs_post("/api/applications", {
            "application": {"name": "Benchmark", "tenantId": tenant_id}
        }, api_key)
        app_id = resp["id"]
        print(f"  Created: {app_id}")

    # Step 4: Device profiles
    print("\n--- Device profiles ---")
    profiles = cs_get("/api/device-profiles?limit=100&tenantId=" + tenant_id, api_key)
    profile_list = profiles.get("result", [])

    existing_abp = find_existing(profile_list, "AM319 ABP")
    if existing_abp:
        abp_profile_id = existing_abp["id"]
        print(f"  ABP found: {abp_profile_id}")
    else:
        abp_resp = cs_post("/api/device-profiles", {
            "deviceProfile": {
                "name": "AM319 ABP", "tenantId": tenant_id,
                "region": "EU868", "macVersion": "LORAWAN_1_0_3",
                "regParamsRevision": "A", "supportsOtaa": False,
                "supportsClassB": False, "supportsClassC": False,
                "payloadCodecRuntime": "JS",
                "payloadCodecScript": AM319_CODEC_DECODE,
                "uplinkInterval": 3600, "deviceStatusReqInterval": 1,
                "flushQueueOnActivate": True, "abpRx1Delay": 1,
                "abpRx1DrOffset": 0, "abpRx2Dr": 0, "abpRx2Freq": 869525000,
            }
        }, api_key)
        abp_profile_id = abp_resp["id"]
        print(f"  ABP created: {abp_profile_id}")

    existing_otaa = find_existing(profile_list, "AM319 OTAA")
    if existing_otaa:
        otaa_profile_id = existing_otaa["id"]
        print(f"  OTAA found: {otaa_profile_id}")
    else:
        otaa_resp = cs_post("/api/device-profiles", {
            "deviceProfile": {
                "name": "AM319 OTAA", "tenantId": tenant_id,
                "region": "EU868", "macVersion": "LORAWAN_1_0_3",
                "regParamsRevision": "A", "supportsOtaa": True,
                "supportsClassB": False, "supportsClassC": False,
                "payloadCodecRuntime": "JS",
                "payloadCodecScript": AM319_CODEC_DECODE,
                "uplinkInterval": 3600, "deviceStatusReqInterval": 1,
                "flushQueueOnActivate": True, "abpRx1Delay": 1,
                "abpRx1DrOffset": 0, "abpRx2Dr": 0, "abpRx2Freq": 869525000,
            }
        }, api_key)
        otaa_profile_id = otaa_resp["id"]
        print(f"  OTAA created: {otaa_profile_id}")

    existing_mcf = find_existing(profile_list, "MCF-LW13IO ABP")
    if existing_mcf:
        mcf_profile_id = existing_mcf["id"]
        print(f"  MCF ABP found: {mcf_profile_id}")
    else:
        mcf_resp = cs_post("/api/device-profiles", {
            "deviceProfile": {
                "name": "MCF-LW13IO ABP", "tenantId": tenant_id,
                "region": "EU868", "macVersion": "LORAWAN_1_0_3",
                "regParamsRevision": "A", "supportsOtaa": False,
                "supportsClassB": False, "supportsClassC": True,
                "payloadCodecRuntime": "JS",
                "payloadCodecScript": MCF_CODEC_DECODE,
                "uplinkInterval": 3600, "deviceStatusReqInterval": 1,
                "flushQueueOnActivate": True, "abpRx1Delay": 1,
                "abpRx1DrOffset": 0, "abpRx2Dr": 0, "abpRx2Freq": 869525000,
            }
        }, api_key)
        mcf_profile_id = mcf_resp["id"]
        print(f"  MCF ABP created: {mcf_profile_id}")

    existing_sdm = find_existing(profile_list, "SDM230 ABP")
    if existing_sdm:
        sdm_profile_id = existing_sdm["id"]
        print(f"  SDM ABP found: {sdm_profile_id}")
    else:
        sdm_resp = cs_post("/api/device-profiles", {
            "deviceProfile": {
                "name": "SDM230 ABP", "tenantId": tenant_id,
                "region": "EU868", "macVersion": "LORAWAN_1_0_3",
                "regParamsRevision": "A", "supportsOtaa": False,
                "supportsClassB": False, "supportsClassC": True,
                "payloadCodecRuntime": "JS",
                "payloadCodecScript": SDM230_CODEC_DECODE,
                "uplinkInterval": 3600, "deviceStatusReqInterval": 1,
                "flushQueueOnActivate": True, "abpRx1Delay": 1,
                "abpRx1DrOffset": 0, "abpRx2Dr": 0, "abpRx2Freq": 869525000,
            }
        }, api_key)
        sdm_profile_id = sdm_resp["id"]
        print(f"  SDM ABP created: {sdm_profile_id}")

    # Step 5: Gateway in ChirpStack
    print("\n--- Gateway (ChirpStack) ---")
    gw_id = "7ef9d3ce197336f1"
    try:
        cs_get(f"/api/gateways/{gw_id}", api_key)
        print(f"  Already exists: {gw_id}")
    except requests.exceptions.HTTPError:
        cs_post("/api/gateways", {
            "gateway": {
                "gatewayId": gw_id, "name": "Benchmark Gateway",
                "tenantId": tenant_id, "statsInterval": 30,
            }
        }, api_key)
        print(f"  Created: {gw_id}")

    # Step 6: Configure simulator
    print("\n--- Simulator ---")

    # Bridge address (uses k8s service name)
    print("  Setting bridge address...")
    sim_post("/bridge/save", {"ip": "chirpstack-gateway-bridge", "port": "1700"})

    # Gateway
    gw_list = requests.get(f"{SIM_URL}/gateways?page=1&limit=100", timeout=10).json()
    if gw_list is None:
        gw_list = []
    elif not isinstance(gw_list, list):
        gw_list = gw_list.get("gateways", [])
    existing_gw = None
    for gw in gw_list:
        info = gw.get("info", gw)
        if info.get("macAddress") == gw_id:
            existing_gw = gw
            break
    if existing_gw:
        print(f"  Gateway already in simulator: {gw_id}")
    else:
        sim_post("/add-gateway", {
            "info": {
                "macAddress": gw_id, "keepAlive": 30, "active": True,
                "typeGateway": False, "name": "Benchmark Gateway",
                "location": {"latitude": 48.2077, "longitude": 16.375, "altitude": 0},
                "ip": "", "port": ""
            }
        })
        print(f"  Gateway added: {gw_id}")

    # Integration (uses k8s service name for internal URL)
    intgs = requests.get(f"{SIM_URL}/integrations", timeout=10).json()
    intg_list = (intgs or {}).get("integrations", []) if isinstance(intgs, dict) else (intgs or [])
    existing_intg = find_existing(intg_list, "Benchmark Chirpstack")
    if existing_intg:
        bench_intg_id = existing_intg["id"]
        print(f"  Integration already exists: {bench_intg_id}")
        sim_post("/update-integration", {
            "id": bench_intg_id,
            "name": "Benchmark Chirpstack",
            "type": "chirpstack",
            "url": "http://chirpstack-rest-api:8090",
            "apiKey": api_key,
            "tenantId": tenant_id,
            "applicationId": app_id,
            "enabled": True
        })
        print(f"  Updated with new API key")
    else:
        for intg in intg_list:
            requests.post(f"{SIM_URL}/delete-integration", json={"id": intg["id"]}, timeout=10)
            print(f"  Deleted old integration: {intg['name']}")

        sim_post("/add-integration", {
            "name": "Benchmark Chirpstack",
            "type": "chirpstack",
            "url": "http://chirpstack-rest-api:8090",
            "apiKey": api_key,
            "tenantId": tenant_id,
            "applicationId": app_id,
            "enabled": True
        })
        intgs = requests.get(f"{SIM_URL}/integrations", timeout=10).json()
        intg_list2 = (intgs or {}).get("integrations", []) if isinstance(intgs, dict) else (intgs or [])
        for intg in intg_list2:
            if intg["name"] == "Benchmark Chirpstack":
                bench_intg_id = intg["id"]
                break
        else:
            print("  ERROR: Could not find integration after creation!")
            sys.exit(1)
        print(f"  Created integration: {bench_intg_id}")

    # Update templates
    template_profiles = [
        (1, "AM319", abp_profile_id),
        (2, "MCF-LW13IO", mcf_profile_id),
        (3, "SDM230", sdm_profile_id),
    ]
    for tmpl_id, tmpl_name, profile_id in template_profiles:
        print(f"  Updating {tmpl_name} template...")
        r = requests.get(f"{SIM_URL}/template/{tmpl_id}", timeout=10)
        tmpl = r.json()
        if isinstance(tmpl, dict) and len(tmpl) == 1:
            key = list(tmpl.keys())[0]
            tmpl = tmpl[key]
        tmpl["integrationEnabled"] = True
        tmpl["integrationId"] = bench_intg_id
        tmpl["deviceProfileId"] = profile_id
        sim_post("/update-template", tmpl)

    print(f"\n--- Seed complete ---")
    print(f"  ChirpStack UI: http://localhost:8080 (admin/admin)")
    print(f"  Simulator UI:  http://localhost:8002/dashboard")
    print(f"  REST API:      http://localhost:8090")
    print(f"  Metrics:       http://localhost:8003/metrics")


if __name__ == "__main__":
    main()
