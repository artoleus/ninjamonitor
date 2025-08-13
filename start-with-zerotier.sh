#!/bin/bash
# start-with-zerotier.sh - Start cloud dashboard with ZeroTier networking

set -e

echo "Starting ZeroTier service..."
zerotier-one -d

# Wait for ZeroTier to initialize
sleep 5

# Join network if ZEROTIER_NETWORK_ID is provided
if [ ! -z "$ZEROTIER_NETWORK_ID" ]; then
    echo "Joining ZeroTier network: $ZEROTIER_NETWORK_ID"
    zerotier-cli join $ZEROTIER_NETWORK_ID
    
    # Wait for network to be ready
    echo "Waiting for ZeroTier network to be ready..."
    for i in {1..30}; do
        if zerotier-cli info | grep -q "200 info"; then
            echo "ZeroTier is ready"
            break
        fi
        echo "Waiting for ZeroTier... ($i/30)"
        sleep 2
    done
    
    # Get ZeroTier IP
    ZT_IP=$(zerotier-cli get $ZEROTIER_NETWORK_ID ip4 2>/dev/null || echo "")
    if [ ! -z "$ZT_IP" ]; then
        echo "ZeroTier IP: $ZT_IP"
        export ZT_IP
    else
        echo "Warning: Could not get ZeroTier IP, using default binding"
    fi
fi

echo "Starting cloud dashboard..."
exec ./cloud-dashboard