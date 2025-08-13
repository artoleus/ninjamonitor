# NinjaMonitor Cloud System

A distributed trading monitoring system with cloud dashboard and local NinjaTrader integration.

## Architecture

### Standard Internet Connection
```
NinjaTrader AddOn → Connection Server (Local) → Cloud Dashboard → Web Clients
                         ↓                            ↑
                    OIF Commands                 WebSocket/HTTP
```

### ZeroTier Secure Network (Recommended)
```
NinjaTrader AddOn → Connection Server (Local) ←──ZeroTier Network──→ Cloud Dashboard → Web Clients
                         ↓                         (Encrypted Tunnel)        ↑
                    OIF Commands                                        WebSocket/HTTP
```

## Components

### 1. Connection Server (`connection-server.go`)
- **Purpose**: Bridge between NinjaTrader and cloud dashboard
- **Location**: Runs locally on Windows machine with NinjaTrader
- **Port**: 8080 (for NinjaTrader AddOn webhooks)
- **Features**: WebSocket client, command processor, OIF file generation

### 2. Cloud Dashboard (`cloud-dashboard.go`)
- **Purpose**: Web-based trading dashboard
- **Location**: Deployed to cloud (Google Cloud Run, Vercel, etc.)
- **Port**: 8081 (configurable via PORT env var)
- **Features**: WebSocket server, web dashboard, command relay

### 3. NinjaTrader AddOn (`TradeBroadcasterAddOn.cs`)
- **Purpose**: Broadcasts account data from NinjaTrader
- **Location**: NinjaTrader 8 AddOns folder
- **Target**: Points to connection server instead of cloud dashboard

## Quick Start

### Local Development
1. **Start Connection Server:**
   ```bash
   CLOUD_URL=ws://localhost:8081/ws go run connection-server.go
   ```

2. **Start Cloud Dashboard:**
   ```bash
   PORT=8081 go run cloud-dashboard.go
   ```

3. **Test System:**
   ```bash
   go run test-system.go
   ```

4. **Access Dashboard:**
   - Open http://localhost:8081 in browser
   - Should show test data and allow commands

### Production Deployment

#### Option A: Standard Internet (Less Secure)
**Connection Server (Local)**
1. Build binary: `go build -o connection-server connection-server.go`
2. Set environment variables:
   - `CLOUD_URL`: WebSocket URL of cloud dashboard
   - `NT_INCOMING`: NinjaTrader incoming folder path
3. Configure firewall/port forwarding for external access

**Cloud Dashboard**
1. Deploy to Google Cloud Run:
   ```bash
   gcloud run deploy ninjamonitor --source . --platform managed
   ```
2. Configure SSL/TLS for WebSocket connections

#### Option B: ZeroTier Network (Recommended)
**Setup ZeroTier Network**
1. Create network at [ZeroTier Central](https://my.zerotier.com)
2. Note your Network ID
3. Configure as private network with custom subnet

**Connection Server (Local)**
```bash
# Install ZeroTier and join network
zerotier-cli join YOUR_NETWORK_ID

# Set environment variables
CLOUD_URL=ws://[ZT-CLOUD-IP]:8081/ws
NT_INCOMING=C:\Users\...\Documents\NinjaTrader 8\incoming

# Run connection server
./connection-server.exe
```

**Cloud Dashboard (VPS/Dedicated Server)**
```bash
# Install ZeroTier and join network
sudo zerotier-cli join YOUR_NETWORK_ID

# Build and run with ZeroTier support
docker build -f Dockerfile.zerotier -t ninjamonitor-zt .
docker run --privileged --env ZEROTIER_NETWORK_ID=YOUR_NETWORK_ID ninjamonitor-zt
```

See `zerotier-config.md` for detailed setup instructions.

#### NinjaTrader AddOn
1. Update `endpointUrl` in TradeBroadcasterAddOn.cs:
   ```csharp
   private string endpointUrl = "http://localhost:8080/webhook";
   ```
2. Compile and install in NinjaTrader

## Environment Variables

### Connection Server
- `CLOUD_URL`: Cloud dashboard WebSocket endpoint (default: ws://localhost:8081/ws)
- `NT_INCOMING`: NinjaTrader incoming folder (default: ~/Documents/NinjaTrader 8/incoming)

### Cloud Dashboard
- `PORT`: HTTP server port (default: 8081)

## Stability Features

### Race Condition Prevention
- Mutex-protected map operations (same as original fix)
- Atomic operations for connection state management
- Proper channel buffering to prevent deadlocks

### Connection Resilience
- Exponential backoff for WebSocket reconnection
- Circuit breaker pattern for failed connections
- Message queuing during network issues
- Heartbeat system for connection health

### Error Recovery
- Command acknowledgment system
- Fallback mechanisms for network failures
- Comprehensive logging for debugging

## Commands

All emergency commands from original system are supported:
- Flatten All Accounts
- Flatten Specific Account  
- Close Position
- Cancel Order

Commands are relayed from cloud dashboard to connection server via WebSocket, then executed locally via OIF files.

## Security Considerations

- Use HTTPS/WSS in production
- Implement authentication for cloud dashboard
- Firewall rules for connection server
- No sensitive data stored in cloud

## Monitoring

- Connection status displayed in dashboard
- Server logs for debugging
- WebSocket connection health monitoring
- Command execution acknowledgments