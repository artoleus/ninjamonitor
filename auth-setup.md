# Authentication Setup for NinjaMonitor

## Current Status ✅

**Dashboard Access**: https://ninjamonitor-dashboard-1016402392245.us-central1.run.app
- ✅ Your Google account (tim@welsted.com) has invoker access
- ✅ Dashboard loads correctly with authentication
- ✅ You can access it directly in your browser

## Browser Access 🌐

Simply visit the URL in your browser:
https://ninjamonitor-dashboard-1016402392245.us-central1.run.app

Your browser will handle Google OAuth automatically since your account has permissions.

## Connection Server Authentication 🔌

For the local connection server to connect via WebSocket, you have **two options**:

### Option 1: Use Your Personal Credentials (Quick Setup)
```bash
# Windows - Set environment variable for gcloud auth
set GOOGLE_APPLICATION_CREDENTIALS=""
set CLOUD_URL=wss://ninjamonitor-dashboard-1016402392245.us-central1.run.app/ws

# Run gcloud auth to get token (you'll need to run this periodically as tokens expire)
FOR /F "tokens=*" %i IN ('gcloud auth print-identity-token') DO SET AUTH_TOKEN=%i

# The connection server would need to be updated to use this token
```

### Option 2: Service Account (Production Setup)
```bash
# Create service account
gcloud iam service-accounts create ninjamonitor-connection --display-name="NinjaMonitor Connection Server"

# Grant Cloud Run invoker role
gcloud run services add-iam-policy-binding ninjamonitor-dashboard \
  --region=us-central1 \
  --member="serviceAccount:ninjamonitor-connection@golden-plateau-468017-d6.iam.gserviceaccount.com" \
  --role="roles/run.invoker"

# Create key file
gcloud iam service-accounts keys create ninjamonitor-key.json \
  --iam-account=ninjamonitor-connection@golden-plateau-468017-d6.iam.gserviceaccount.com

# Set environment variables on Windows
set GOOGLE_APPLICATION_CREDENTIALS=ninjamonitor-key.json
set CLOUD_URL=wss://ninjamonitor-dashboard-1016402392245.us-central1.run.app/ws
```

## Current Connection Server Limitations

The current `connection-server.go` doesn't include authentication for WebSocket connections. You have two choices:

### Choice A: Update Connection Server with Authentication
- Modify the WebSocket dial to include Google Cloud authentication
- Add Google Cloud SDK dependencies to the Go application
- Handle token refresh automatically

### Choice B: Use ZeroTier with VPS (Original Plan)
- Deploy dashboard to a VPS instead of Cloud Run
- Use ZeroTier for secure networking
- No authentication complexity

## Quick Test

You can test the dashboard immediately by visiting the URL in your browser:
https://ninjamonitor-dashboard-1016402392245.us-central1.run.app

The dashboard will show "Connected: disconnected" since no connection server is connected yet, but you can see the full interface.

## Next Steps

Which approach would you prefer:
1. **Update connection server code** to handle Google Cloud authentication
2. **Switch to ZeroTier + VPS** for simpler networking
3. **Use for testing only** and connect later

The dashboard is fully functional and waiting for a connection server!