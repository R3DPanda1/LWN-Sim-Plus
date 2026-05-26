#!/bin/sh
# Raise the kernel UDP receive buffer so chirpstack-gateway-bridge can absorb
# uplink bursts from the simulator without RcvbufErrors.  Default rmem
# (~208 KiB) fits ~700 PUSH_DATA packets, which overflows at tiers of a few
# thousand devices with long send intervals.  16 MiB fits ~55k packets.
set -eu
sudo sysctl -w net.core.rmem_max=16777216
sudo sysctl -w net.core.rmem_default=16777216
sudo sysctl -w net.core.netdev_max_backlog=10000

cat <<'EOF' | sudo tee /etc/sysctl.d/99-lwnsim-udp-buffers.conf
net.core.rmem_max = 16777216
net.core.rmem_default = 16777216
net.core.netdev_max_backlog = 10000
EOF
echo "Applied and persisted.  Restart the gateway-bridge container for the new default to take effect."
