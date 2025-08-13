# Mobile Access to NinjaMonitor Dashboard

## Current Situation 📱

**Desktop Access**: ✅ Working via gcloud proxy at http://localhost:8082  
**Mobile Access**: ❌ Blocked by organization security policy  
**Your Network**: Local machine IP `192.168.86.247`  

## Mobile Access Solutions

### Option 1: SSH Tunnel (Recommended) 🔐

**Setup SSH tunnel from your mobile to desktop:**

1. **Enable SSH on your Linux desktop** (if not already enabled):
```bash
sudo systemctl enable ssh
sudo systemctl start ssh
```

2. **From mobile SSH app** (like Termux on Android or SSH client on iOS):
```bash
# Create tunnel: mobile port 8082 → desktop port 8082
ssh -L 8082:localhost:8082 tim@192.168.86.247
```

3. **Access on mobile browser**: http://localhost:8082

### Option 2: Reverse Proxy with Authentication 🌐

**Set up nginx or similar on your desktop:**

```nginx
# /etc/nginx/sites-available/ninjamonitor
server {
    listen 80;
    server_name 192.168.86.247;
    
    auth_basic "NinjaMonitor Access";
    auth_basic_user_file /etc/nginx/.htpasswd;
    
    location / {
        proxy_pass http://127.0.0.1:8082;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

**Mobile access**: http://192.168.86.247 (with username/password)

### Option 3: ZeroTier Mobile App 🔗

**Install ZeroTier on mobile and desktop:**

1. **Install ZeroTier One on desktop** (if not already):
```bash
curl -s https://install.zerotier.com | sudo bash
sudo zerotier-cli join 93afae59631a6798
```

2. **Install ZeroTier One mobile app**
3. **Join same network**: 93afae59631a6798
4. **Authorize both devices** in ZeroTier Central
5. **Access via ZeroTier IP** (e.g., http://10.147.17.200:8082)

### Option 4: Override Organization Policy (If Possible) 🔓

**As project owner, you might be able to enable public access:**

```bash
# Try to remove the organization policy constraint
gcloud resource-manager org-policies delete \
    constraints/run.allowedIngress \
    --organization=217702371523
```

**Then enable public access:**
```bash
gcloud run services add-iam-policy-binding ninjamonitor-dashboard \
    --region=us-central1 \
    --member="allUsers" \
    --role="roles/run.invoker"
```

### Option 5: VPS Deployment 🚀

**Deploy to VPS with public access enabled:**

1. **Deploy to DigitalOcean/Linode/etc.**
2. **No authentication complexity**
3. **Direct HTTPS access from anywhere**
4. **Use with ZeroTier for secure connection server**

## Recommended Approach 🎯

**For immediate access**: Option 1 (SSH Tunnel)  
**For permanent solution**: Option 3 (ZeroTier) or Option 5 (VPS)

## Current Status

**Desktop proxy running**: http://localhost:8082  
**Mobile access**: Choose one of the options above

### Quick SSH Tunnel Test

If you have SSH enabled on your Linux machine, try this from your mobile:

1. **Download SSH app** (Termux, ConnectBot, etc.)
2. **Connect**: `ssh tim@192.168.86.247`
3. **In another session/tab, create tunnel**:
   ```bash
   ssh -L 8082:localhost:8082 tim@192.168.86.247
   ```
4. **Open mobile browser**: http://localhost:8082

This should give you immediate mobile access to your NinjaMonitor dashboard!