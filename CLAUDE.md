# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

NinjaMonitor is a distributed real-time trading monitoring system with cloud deployment capability. It consists of three main components:

1. **Connection Server** (`connection-server.go`) - Local bridge between NinjaTrader and cloud
2. **Cloud Dashboard** (`cloud-dashboard.go`) - Web-based dashboard deployable to cloud platforms
3. **NinjaTrader AddOn** (`TradeBroadcasterAddOn.cs`) - C# plugin that broadcasts account data from NinjaTrader

## Architecture

### Data Flow
- NinjaTrader AddOn monitors account events (orders, positions, executions)
- AddOn sends JSON snapshots via HTTP POST to Connection Server's `/webhook` endpoint (port 8080)
- Connection Server relays data to Cloud Dashboard via WebSocket connection
- Cloud Dashboard broadcasts to web clients via Server-Sent Events
- Commands flow reverse: Web Dashboard → Cloud Dashboard → Connection Server → OIF files

### Key Components
- **Position/Order Structs**: Data models shared across all components
- **WebSocket Communication**: Bidirectional real-time communication between connection server and cloud
- **Command Relay System**: Commands sent from cloud dashboard to local execution
- **Stability Patterns**: Race condition fixes and throttling mechanisms from original solution

## Development Commands

### Local Development Setup
```bash
# Start connection server (local bridge)
CLOUD_URL=ws://localhost:8081/ws go run connection-server.go

# Start cloud dashboard (separate terminal)
PORT=8081 go run cloud-dashboard.go

# Test the system
go run test-system.go

# Build binaries
go build -o connection-server connection-server.go
go build -o cloud-dashboard cloud-dashboard.go
```

### Production Deployment
```bash
# Connection server (local Windows machine)
go build -o connection-server.exe connection-server.go
CLOUD_URL=wss://your-cloud-domain.com/ws ./connection-server.exe

# Cloud dashboard (deploy to cloud platform)
go build -o cloud-dashboard cloud-dashboard.go
# Deploy to Google Cloud Run, Vercel, etc.
```

### NinjaTrader AddOn
The C# AddOn must be compiled within NinjaTrader 8's development environment. Update the `endpointUrl` to point to connection server:
```csharp
private string endpointUrl = "http://localhost:8080/webhook";
```

## Configuration

### Connection Server Environment Variables
- `CLOUD_URL`: WebSocket URL of cloud dashboard (default: ws://localhost:8081/ws)
- `NT_INCOMING`: NinjaTrader incoming folder path (default: `~/Documents/NinjaTrader 8/incoming`)

### Cloud Dashboard Environment Variables  
- `PORT`: Web server port (default: 8081)

### AddOn Configuration
- `endpointUrl`: Points to connection server webhook endpoint (port 8080)
- `ThrottleTimeMs`: Update throttling interval in milliseconds (default: 250ms)

## Key API Endpoints

### Data Endpoints
- `GET /`: Web dashboard interface
- `GET /events`: Server-Sent Events stream for real-time updates
- `POST /webhook`: Receives account snapshots from NinjaTrader AddOn

### Command Endpoints (Emergency Controls)
- `POST /api/flatten`: Flatten all positions across all accounts
- `POST /api/flatten_account`: Flatten specific account
- `POST /api/close_position`: Close specific position
- `POST /api/cancel_order`: Cancel specific order

## Security Considerations
- Server binds to `0.0.0.0` - consider firewall rules for production
- Emergency flatten commands have confirmation dialogs but no authentication
- AddOn uses HTTP (not HTTPS) by default for local communication