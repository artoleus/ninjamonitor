# NinjaMonitor Cloud Deployment Guide

## Project Configuration ✅

**Google Cloud Project**: `golden-plateau-468017-d6` ("My First Project")
**ZeroTier Network**: `93afae59631a6798`

## Cloud Run Limitations with ZeroTier

**Important**: Google Cloud Run has limitations that prevent ZeroTier from working properly:
- No privileged container mode
- Stateless containers that restart frequently
- Limited networking capabilities

## Recommended Deployment Options

### Option 1: VPS with ZeroTier (Recommended for Security)

**1. Deploy to VPS (DigitalOcean, Linode, etc.)**
```bash
# On Ubuntu/Debian VPS
sudo apt update && sudo apt install -y docker.io

# Install ZeroTier
curl -s https://install.zerotier.com | sudo bash
sudo zerotier-cli join 93afae59631a6798

# Build and run NinjaMonitor
git clone https://github.com/artoleus/ninjamonitor.git
cd ninjamonitor
docker build -f Dockerfile.zerotier -t ninjamonitor-zerotier .
docker run -d \
  --name ninjamonitor \
  --network host \
  --privileged \
  -e ZEROTIER_NETWORK_ID=93afae59631a6798 \
  -p 8081:8081 \
  ninjamonitor-zerotier
```

**2. Authorize VPS in ZeroTier Central**
- Go to https://my.zerotier.com
- Find network `93afae59631a6798`
- Authorize the new VPS device
- Note the assigned IP (e.g., `10.147.17.100`)

### Option 2: Standard Cloud Run (Public Internet)

**Deploy basic version to Cloud Run:**
```bash
# Set project
gcloud config set project golden-plateau-468017-d6

# Deploy (without ZeroTier)
gcloud run deploy ninjamonitor-dashboard \
  --source . \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --memory 512Mi \
  --cpu 1
```

**Update connection server to use public Cloud Run URL:**
```bash
# Use HTTPS URL provided by Cloud Run
set CLOUD_URL=wss://ninjamonitor-dashboard-xxx-uc.a.run.app/ws
connection-server.exe
```

## Local Connection Server Setup

### For VPS with ZeroTier (Secure)
```bash
# Windows Command Prompt
set CLOUD_URL=ws://10.147.17.100:8081/ws
set NT_INCOMING=C:\Users\%USERNAME%\Documents\NinjaTrader 8\incoming
connection-server.exe
```

### For Cloud Run (Public Internet)
```bash
# Windows Command Prompt - Use HTTPS/WSS URL from Cloud Run
set CLOUD_URL=wss://ninjamonitor-dashboard-xxx-uc.a.run.app/ws
set NT_INCOMING=C:\Users\%USERNAME%\Documents\NinjaTrader 8\incoming
connection-server.exe
```

## NinjaTrader AddOn Configuration

Update `TradeBroadcasterAddOn.cs` to point to your connection server:
```csharp
private string endpointUrl = "http://localhost:8080/webhook";
```

## Security Comparison

| Method | Security Level | Setup Complexity | Cost |
|--------|---------------|------------------|------|
| **VPS + ZeroTier** | 🟢 High (Encrypted tunnel) | 🟡 Medium | 💰 Low ($5-20/month) |
| **Cloud Run Public** | 🟡 Medium (HTTPS only) | 🟢 Easy | 💰 Very Low (Pay per use) |

## Next Steps

1. **Choose deployment method** based on security requirements
2. **Deploy cloud dashboard** using chosen method
3. **Update connection server** with correct CLOUD_URL
4. **Authorize devices** in ZeroTier Central (if using VPS)
5. **Test connection** between components

## Troubleshooting

### Cloud Run Issues
- Check service logs: `gcloud logging read "resource.type=cloud_run_revision" --limit 50`
- Verify buildpack detects Go: Ensure `go.mod` exists
- Check memory/CPU limits

### ZeroTier VPS Issues
- Verify authorized in network: `sudo zerotier-cli listnetworks`
- Check IP assignment: `ip addr show zt+`
- Test connectivity: `ping [other-device-ip]`

### Connection Server Issues
- Verify CLOUD_URL format: `ws://` for HTTP, `wss://` for HTTPS
- Check NinjaTrader incoming folder path
- Test with curl: `curl -I [webhook-url]`