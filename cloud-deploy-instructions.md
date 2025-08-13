# Google Cloud Run Deployment Instructions

## Prerequisites Setup

Your Google Cloud project needs these APIs enabled:

1. **Enable Required APIs**: Visit these URLs and enable the APIs:
   - Cloud Run API: https://console.developers.google.com/apis/api/run.googleapis.com/overview?project=golden-plateau-468017-d6
   - Artifact Registry API: https://console.developers.google.com/apis/api/artifactregistry.googleapis.com/overview?project=golden-plateau-468017-d6
   - Cloud Build API: https://console.developers.google.com/apis/api/cloudbuild.googleapis.com/overview?project=golden-plateau-468017-d6

## Automated Deployment

Once APIs are enabled, run this command:

```bash
gcloud run deploy ninjamonitor-dashboard \
  --source . \
  --platform managed \
  --region us-central1 \
  --port 8081 \
  --set-env-vars "PORT=8081,BIND_ADDRESS=0.0.0.0,ZEROTIER_NETWORK_ID=93afae59631a6798" \
  --allow-unauthenticated \
  --max-instances 5 \
  --memory 1Gi \
  --cpu 1 \
  --timeout 3600 \
  --dockerfile Dockerfile.zerotier
```

## ZeroTier Network Configuration

### 1. Authorize the Cloud Run Instance
After deployment, you need to authorize the Cloud Run instance in your ZeroTier network:

1. Go to [ZeroTier Central](https://my.zerotier.com)
2. Navigate to your network: `93afae59631a6798`
3. Look for a new device (the Cloud Run instance)
4. Check the "Authorized" box for this device
5. Note the assigned IP address (e.g., `10.147.17.xxx`)

### 2. Update Connection Server Configuration
On your local Windows machine, update the connection server configuration:

```bash
# Set the ZeroTier IP of your Cloud Run instance
set CLOUD_URL=ws://10.147.17.xxx:8081/ws
set NT_INCOMING=C:\Users\YourUser\Documents\NinjaTrader 8\incoming

# Run connection server
connection-server.exe
```

## Manual Deployment Steps

If the automated deployment doesn't work:

### 1. Build and Push Image
```bash
# Configure Docker authentication
gcloud auth configure-docker

# Build image
docker build -f Dockerfile.zerotier -t gcr.io/golden-plateau-468017-d6/ninjamonitor-zerotier .

# Push to registry
docker push gcr.io/golden-plateau-468017-d6/ninjamonitor-zerotier
```

### 2. Deploy to Cloud Run
```bash
gcloud run deploy ninjamonitor-dashboard \
  --image gcr.io/golden-plateau-468017-d6/ninjamonitor-zerotier \
  --platform managed \
  --region us-central1 \
  --port 8081 \
  --set-env-vars "PORT=8081,BIND_ADDRESS=0.0.0.0,ZEROTIER_NETWORK_ID=93afae59631a6798" \
  --allow-unauthenticated \
  --max-instances 5 \
  --memory 1Gi \
  --cpu 1 \
  --timeout 3600
```

## Important Notes

### ZeroTier Limitations in Cloud Run
- Cloud Run containers are stateless and restart frequently
- ZeroTier needs to rejoin the network on each restart
- Container needs privileged mode (not supported in Cloud Run)

### Alternative: Use a VPS instead
For better ZeroTier support, consider using a dedicated VPS:

```bash
# On Ubuntu/Debian VPS
sudo apt update && sudo apt install -y curl

# Install ZeroTier
curl -s https://install.zerotier.com | sudo bash

# Join network
sudo zerotier-cli join 93afae59631a6798

# Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Run NinjaMonitor with ZeroTier
docker run -d \
  --name ninjamonitor \
  --network host \
  --privileged \
  -e ZEROTIER_NETWORK_ID=93afae59631a6798 \
  gcr.io/golden-plateau-468017-d6/ninjamonitor-zerotier
```

## Testing the Deployment

### 1. Verify Cloud Dashboard
- Access the Cloud Run URL in browser
- Should show NinjaMonitor dashboard interface

### 2. Test ZeroTier Connectivity
```bash
# From your local connection server machine
ping [CLOUD-RUN-ZEROTIER-IP]
```

### 3. Test WebSocket Connection
- Run connection server locally with CLOUD_URL pointing to ZeroTier IP
- Should see "Connected to cloud dashboard" in logs
- Dashboard should show real-time data from NinjaTrader

## Troubleshooting

### Container Startup Issues
Check Cloud Run logs:
```bash
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=ninjamonitor-dashboard" --limit 50
```

### ZeroTier Network Issues
- Verify device is authorized in ZeroTier Central
- Check network configuration allows traffic on port 8081
- Ensure WebSocket traffic isn't blocked by firewall

### Connection Server Issues
- Verify CLOUD_URL points to correct ZeroTier IP
- Check Windows firewall allows outbound connections
- Test basic connectivity with ping/telnet