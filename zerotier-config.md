# ZeroTier Network Configuration for NinjaMonitor

## Overview
Using ZeroTier creates a secure, encrypted overlay network between your cloud dashboard and local connection server without exposing any ports to the public internet.

## Network Architecture
```
┌─────────────────────────────────┐    ZeroTier Network     ┌─────────────────────────────────┐
│      Cloud Dashboard            │    (10.147.x.x/24)     │    Connection Server           │
│   (Google Cloud Run/VPS)       │◄─────────────────────────►│      (Local Windows)           │
│   ZT IP: 10.147.17.100         │                         │    ZT IP: 10.147.17.200       │
└─────────────────────────────────┘                         └─────────────────────────────────┘
                                                                             │
                                                                             ▼
                                                            ┌─────────────────────────────────┐
                                                            │        NinjaTrader 8            │
                                                            │       (localhost:8080)         │
                                                            └─────────────────────────────────┘
```

## Setup Instructions

### 1. Create ZeroTier Network
1. Log into [ZeroTier Central](https://my.zerotier.com)
2. Create new network (note the Network ID)
3. Configure network settings:
   - **Access Control**: Private
   - **IPv4 Assignment**: 10.147.17.0/24 (or your preferred subnet)
   - **Allow Default Route**: No (for security)

### 2. Install ZeroTier on Connection Server (Windows)
```bash
# Download and install ZeroTier One from zerotier.com
# Join your network
zerotier-cli join YOUR_NETWORK_ID

# Check status
zerotier-cli status
zerotier-cli listnetworks
```

### 3. Cloud Dashboard Setup

#### Option A: Google Cloud Run with ZeroTier
Create Dockerfile for cloud dashboard:
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o cloud-dashboard cloud-dashboard.go

FROM alpine:latest
RUN apk add --no-cache curl bash
# Install ZeroTier
RUN curl -s https://install.zerotier.com | sh

WORKDIR /app
COPY --from=builder /app/cloud-dashboard .
COPY start-with-zerotier.sh .
RUN chmod +x start-with-zerotier.sh

EXPOSE 8081
CMD ["./start-with-zerotier.sh"]
```

#### Option B: VPS/Dedicated Server (Recommended)
```bash
# Install ZeroTier on cloud server
curl -s https://install.zerotier.com | sudo bash

# Join network
sudo zerotier-cli join YOUR_NETWORK_ID

# Start cloud dashboard bound to ZeroTier interface
ZT_IP=$(zerotier-cli get YOUR_NETWORK_ID ip4)
./cloud-dashboard
```

## Configuration Updates

### Connection Server Environment Variables
```bash
# Use ZeroTier IP of cloud dashboard
CLOUD_URL=ws://10.147.17.100:8081/ws
NT_INCOMING=C:\Users\YourUser\Documents\NinjaTrader 8\incoming
```

### Cloud Dashboard Environment Variables
```bash
# Bind to all interfaces (including ZeroTier)
PORT=8081
BIND_ADDRESS=0.0.0.0
```

### Modified Connection Server Code
Update connection-server.go to support ZeroTier networking:

```go
func NewConnectionServer() *ConnectionServer {
    cloudURL := os.Getenv("CLOUD_URL")
    if cloudURL == "" {
        // Try to auto-detect ZeroTier IP
        if ztIP := getZeroTierIP(); ztIP != "" {
            cloudURL = fmt.Sprintf("ws://%s:8081/ws", ztIP)
        } else {
            cloudURL = "ws://localhost:8081/ws"
        }
    }
    // ... rest of constructor
}

func getZeroTierIP() string {
    // Platform-specific ZeroTier IP detection
    // Linux: ip route show dev zt+ | head -1 | awk '{print $7}'
    // Windows: More complex - could parse zerotier-cli output
    return ""
}
```

## Security Benefits

### Network Isolation
- ✅ No public IP exposure for connection server
- ✅ Encrypted communication between cloud and local
- ✅ NinjaTrader remains on local network only
- ✅ Fine-grained access control via ZeroTier rules

### Access Control Rules (ZeroTier Central)
```json
{
  "rules": [
    {
      "action": "accept",
      "src": {"zt": "10.147.17.100"},
      "dst": {"zt": "10.147.17.200", "port": 8080}
    },
    {
      "action": "accept", 
      "src": {"zt": "10.147.17.200"},
      "dst": {"zt": "10.147.17.100", "port": 8081}
    },
    {
      "action": "drop"
    }
  ]
}
```

## Deployment Checklist

### Local Setup (Windows)
- [ ] Install ZeroTier One
- [ ] Join your network
- [ ] Authorize device in ZeroTier Central
- [ ] Note assigned ZeroTier IP
- [ ] Update CLOUD_URL environment variable
- [ ] Test connection to cloud dashboard

### Cloud Setup
- [ ] Deploy dashboard with ZeroTier support
- [ ] Join same network
- [ ] Authorize device in ZeroTier Central  
- [ ] Verify WebSocket connection works
- [ ] Test command relay functionality

## Troubleshooting

### Connection Issues
```bash
# Check ZeroTier status
zerotier-cli status

# Test connectivity
ping 10.147.17.100  # From connection server
ping 10.147.17.200  # From cloud dashboard

# Check network membership
zerotier-cli listnetworks
```

### Common Problems
- **Device not authorized**: Check ZeroTier Central
- **Wrong IP address**: Verify assigned IPs match configuration
- **Firewall blocking**: ZeroTier uses UDP 9993
- **Container networking**: Ensure privileged mode for Docker

## Advantages Over Public Internet
1. **Security**: Encrypted, private network tunnel
2. **Simplicity**: No port forwarding or firewall configuration
3. **Reliability**: Direct peer-to-peer when possible
4. **Flexibility**: Easy to add more devices to network
5. **Monitoring**: Network activity visible in ZeroTier Central