# LWN-Sim-Plus

[![GitHub license](https://img.shields.io/github/license/R3DPanda1/LWN-Sim-Plus)](https://github.com/R3DPanda1/LWN-Sim-Plus/blob/main/LICENSE.txt)
[![made-with-Go](https://img.shields.io/badge/Made%20with-Go-1f425f.svg)](https://golang.org)

**Advanced LoRaWAN Device Simulator with JavaScript Codecs and State Management**

Master's Thesis Project - An enhanced fork of [LWN-Simulator](https://github.com/UniCT-ARSLab/LWN-Simulator) with dynamic payload generation, stateful codec execution, and production-ready Kubernetes deployment.

## Key Enhancements Over LWN-Simulator

- **JavaScript Codec Support**: ChirpStack-compatible payload encoder/decoder system
- **Stateful Device Simulation**: Persistent counters, variables, and message history
- **Multi-Part Message Support**: Stateful payload generation for complex scenarios
- **Production-Ready**: Kubernetes manifests, health checks, Prometheus metrics
- **Scalable**: Optimized for 100+ concurrent devices
- **Device Profiles**: Pre-configured templates for real LoRaWAN devices

## Table of Contents

* [What's New](#whats-new)
* [Quick Start](#quick-start)
* [JavaScript Codec System](#javascript-codec-system)
* [Device Profiles](#device-profiles)
* [Kubernetes Deployment](#kubernetes-deployment)
* [Requirements](#requirements)
* [Installation](#installation)
* [Original LWN-Simulator Features](#original-lwn-simulator-features)
* [Thesis Information](#thesis-information)

## What's New

### JavaScript Codec System

Execute ChirpStack-compatible JavaScript codecs server-side for dynamic payload generation:

```javascript
function Encode(fPort, obj) {
    // Access persistent state
    var counter = getCounter("messageCount");
    setCounter("messageCount", counter + 1);

    // Build payload
    var bytes = [];
    bytes.push((obj.temperature + 50) * 2);
    bytes.push(obj.humidity);
    bytes.push((counter >> 8) & 0xFF);
    bytes.push(counter & 0xFF);

    return bytes;
}

function Decode(fPort, bytes) {
    return {
        temperature: (bytes[0] / 2) - 50,
        humidity: bytes[1],
        counter: (bytes[2] << 8) | bytes[3]
    };
}
```

**Built-in Helper Functions:**
- `getCounter(name)` / `setCounter(name, value)` - Persistent counters
- `getState(name)` / `setState(name, value)` - Custom state variables
- `getPreviousPayload()` / `getPreviousPayloads(n)` - Message history
- `log(message)` - Debug logging

### Stateful Device Simulation

- **Persistent State**: Counters and variables survive restarts
- **Message History**: Access previous N payloads
- **Multi-Part Messages**: Stateful payload generation with automatic sequencing

### Production Kubernetes Deployment

```bash
kubectl apply -f k8s/
```

Includes:
- StatefulSet with persistent storage
- Health and readiness probes
- Prometheus metrics endpoint
- ConfigMap-based configuration
- Resource limits and requests

## Quick Start

### Using Docker Compose (with ChirpStack)

```bash
# Start the entire stack
docker compose up -d

# Access the simulator
open http://localhost:8000
```

### From Source

```bash
# Clone the repository
git clone https://github.com/R3DPanda1/LWN-Sim-Plus.git
cd LWN-Sim-Plus

# Install dependencies
make install-dep

# Build
make build

# Run
./bin/lwn-sim-plus
```

## JavaScript Codec System

### Creating a Codec

1. Open the web UI at `http://localhost:8000`
2. Create or select a device
3. Click "Configure Codec"
4. Write your JavaScript codec (ChirpStack-compatible format)
5. Test and save

### Example: Multi-Part Messages

```javascript
function Encode(fPort, obj) {
    var partIndex = getCounter("partIndex");
    var totalParts = 3;

    // Generate large payload that needs splitting
    var fullMessage = [];
    for (var i = 0; i < 150; i++) {
        fullMessage.push((i + obj.value) & 0xFF);
    }

    var maxSize = 51; // LoRaWAN max payload for SF7

    var start = partIndex * maxSize;
    var part = fullMessage.slice(start, start + maxSize);

    // Increment for next message
    setCounter("partIndex", (partIndex + 1) % totalParts);

    // Add fragmentation header
    return [partIndex, totalParts].concat(part);
}
```

## Device Profiles

Pre-configured device profiles with real-world codecs:

- **Dragino LHT65**: Temperature & humidity sensor
- **Browan Temperature Sensor**: Simple sensor
- **Milesight AM107**: Counter tracking
- **Custom Multi-Part**: Complex payload scenarios

Import from The Things Network codec library or create your own.

## Kubernetes Deployment

Deploy to production Kubernetes cluster:

```bash
# Apply all manifests
kubectl apply -f k8s/

# Check status
kubectl get pods -n lorawan-simulator

# Access via port-forward
kubectl port-forward svc/lwn-sim-plus 8000:8000 -n lorawan-simulator
```

See [k8s/README.md](k8s/README.md) for detailed deployment guide.

## Requirements

* **Go**: Version >= 1.21
* **ChirpStack**: For testing (optional, Docker Compose included)
* **Kubernetes**: For production deployment (optional)

## Installation

### From Binary File

Download pre-compiled binaries from the [Releases Page](https://github.com/R3DPanda1/LWN-Sim-Plus/releases)

### From Source Code

#### Build Steps

```bash
# Clone repository
git clone https://github.com/R3DPanda1/LWN-Sim-Plus.git
cd LWN-Sim-Plus

# Install dependencies
make install-dep

# Build
make build

# Run
./bin/lwn-sim-plus  # Linux
./bin/lwn-sim-plus.exe  # Windows
```

### Configuration File

The simulator uses `config.json` for configuration:

```json
{
  "address": "0.0.0.0",
  "port": 8000,
  "metricsPort": 8001,
  "configDirname": "lwnsimulator",
  "autoStart": false,
  "verbose": true
}
```

## Original LWN-Simulator Features

This project is based on [LWN-Simulator](https://github.com/UniCT-ARSLab/LWN-Simulator) and retains all original features:

### The Device

* Based on [LoRaWAN specification v1.0.3](https://lora-alliance.org/resource_hub/lorawan-specification-v1-0-3/)
* Supports all [LoRaWAN Regional Parameters v1.0.3](https://lora-alliance.org/resource_hub/lorawan-regional-parameters-v1-0-3reva/)
* Implements Class A, C, and partial Class B
* ADR (Adaptive Data Rate) Algorithm
* MAC Commands support
* FPending procedure
* Real-time interaction via web UI

### The Forwarder

Receives frames from devices, creates RXPK objects, and forwards to gateways.

### The Gateway

Two types supported:
* **Virtual gateway**: Communicates with real gateway bridges
* **UDP gateway**: Receives/sends UDP datagrams

## Thesis Information

**Project**: LWN-Sim-Plus - Advanced LoRaWAN Device Simulator
**Type**: Master's Thesis Project
**Timeline**: December 2024 - April 2025
**Author**: Alper Ramadan
**Institution**: [Your University]

### Thesis Objectives

1. Implement server-side JavaScript codec execution for dynamic payload generation
2. Design stateful device simulation with persistent state management
3. Support complex scenarios (multi-part messages, stateful payloads)
4. Achieve scalability targets (100+ concurrent devices)
5. Provide production-ready Kubernetes deployment

### Documentation

- [Codec Creation Guide](docs/CODEC_GUIDE.md)
- [User Guide](docs/USER_GUIDE.md)
- [API Reference](docs/API.md)
- [Architecture Overview](docs/ARCHITECTURE.md)
- [Performance Tuning](docs/PERFORMANCE.md)

## Performance Benchmarks

| Metric | Target | Status |
|--------|--------|--------|
| Concurrent Devices | 100+ | In Progress |
| Codec Execution (p95) | <10ms | In Progress |
| Memory per Device | <2MB | In Progress |
| CPU (100 devices, 2-core) | <50% | In Progress |

## License

This project inherits the MIT License from LWN-Simulator.

## Acknowledgments

- Original [LWN-Simulator](https://github.com/UniCT-ARSLab/LWN-Simulator) by UniCT-ARSLab
- [ChirpStack](https://www.chirpstack.io/) for LoRaWAN Network Server
- [The Things Network](https://www.thethingsnetwork.org/) for codec library inspiration
- [goja](https://github.com/dop251/goja) for JavaScript runtime

## Publications Citing Original LWN-Simulator

- [LWN Simulator-A LoRaWAN Network Simulator](https://ieeexplore.ieee.org/document/10477816)
- [Lightweight Root Key Management Scheme in Smart Grid IoT Network based on Blockchain Technology](https://www.researchsquare.com/article/rs-3330383/v1)
- [Optimizing LoRa for Edge Computing with TinyML Pipeline for Channel Hopping](https://arxiv.org/abs/2412.01609)

---

**Status**: Active Development - Master's Thesis Project (Dec 2024 - Apr 2025)
