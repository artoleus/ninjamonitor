# NinjaMonitor Cloud System

A distributed trading monitoring system with cloud dashboard and local NinjaTrader integration.

## Architecture

The system consists of two main components that communicate over a secure WebSocket connection:

`NinjaTrader AddOn` → `Connection Server (Local)` ↔ `Cloud Dashboard (Cloud)` → `Web Clients`

- The **Connection Server** runs on your local machine alongside NinjaTrader.
- The **Cloud Dashboard** is deployed to a cloud platform like Railway, Vercel, or any hosting service.
- The connection is secured using a secret token, ensuring that only your authorized Connection Server can connect to your Cloud Dashboard.

## Components

### 1. Cloud Dashboard (`cloud-dashboard.go`)
- **Purpose**: A web-based dashboard to monitor your trading accounts from anywhere.
- **Deployment**: Runs in the cloud.
- **Authentication**:
    - **UI Access**: Protected by a username and password.
    - **API Access**: Secured by a secret token (`API_SECRET_TOKEN`).

### 2. Connection Server (`connection-server.go`)
- **Purpose**: A lightweight bridge that receives data from the NinjaTrader AddOn and forwards it to the Cloud Dashboard. It also receives commands (e.g., "Flatten") from the dashboard and executes them locally.
- **Deployment**: Runs on the same Windows machine as NinjaTrader.
- **Authentication**: Uses the shared secret token (`API_SECRET_TOKEN`) to connect to the Cloud Dashboard.

### 3. NinjaTrader AddOn (`TradeBroadcasterAddOn.cs`)
- **Purpose**: Broadcasts account data from within NinjaTrader to the local Connection Server.
- **Configuration**: The `endpointUrl` in the AddOn should be pointed to your local Connection Server (typically `http://localhost:8080/webhook`).

## Setup and Deployment

### Step 1: Generate a Secure Secret Token
First, generate a strong, random string to use as your `API_SECRET_TOKEN`. You can use a password generator for this. This token will be used to secure the connection between the two server components.

### Step 2: Deploy the Cloud Dashboard
1.  Choose a cloud provider (e.g., Railway, Heroku, Google Cloud Run).
2.  Fork this repository and connect it to your provider.
3.  Set the following environment variables in your cloud provider's dashboard:
    - `API_SECRET_TOKEN`: The secure token you generated in Step 1.
    - `DASHBOARD_USER`: A username for logging into the web dashboard.
    - `DASHBOARD_PASS`: A password for logging into the web dashboard.
    - `PORT`: The port your provider expects the app to listen on (e.g., `8080`). Your provider often sets this automatically.
4.  Deploy the application. Your provider will give you a public URL (e.g., `https://my-dashboard.dev`).

### Step 3: Run the Connection Server Locally
1.  On your Windows machine where NinjaTrader is running, build the Connection Server:
    ```bash
    go build -o connection-server.exe connection-server.go
    ```
2.  Set the required environment variables in your terminal or system settings:
    - `API_SECRET_TOKEN`: The **same** secure token you used for the Cloud Dashboard.
    - `CLOUD_URL`: The WebSocket URL of your deployed Cloud Dashboard. **Remember to use `wss://` for a secure connection.** For example: `wss://my-dashboard.dev/ws`.
    - `NT_INCOMING`: The full path to your NinjaTrader "incoming" folder (e.g., `C:\Users\YourUser\Documents\NinjaTrader 8\incoming`).
3.  Run the server:
    ```bash
    ./connection-server.exe
    ```

## Local Development
1.  **Generate a secret token** (any string will do for local testing, e.g., `dev-secret-token`).

2.  **Terminal 1: Start Cloud Dashboard**
    ```bash
    export API_SECRET_TOKEN="dev-secret-token"
    export DASHBOARD_USER="admin"
    export DASHBOARD_PASS="password"
    export PORT="8081"
    go run cloud-dashboard.go
    ```

3.  **Terminal 2: Start Connection Server**
    ```bash
    export API_SECRET_TOKEN="dev-secret-token"
    export CLOUD_URL="ws://localhost:8081/ws"
    go run connection-server.go
    ```

4.  **Access the dashboard** at `http://localhost:8081` and log in with `admin`/`password`.

5.  **(Optional) Run the Test System** to send sample data:
    ```bash
    go run test-system.go
    ```

## Environment Variables

### Cloud Dashboard
- `API_SECRET_TOKEN` (**Required**): The shared secret for securing the WebSocket connection.
- `DASHBOARD_USER` (Optional, default: `admin`): Username for the web UI.
- `DASHBOARD_PASS` (Optional, default: `ninja123`): Password for the web UI.
- `PORT` (Optional, default: `8081`): Port for the web server to listen on.

### Connection Server
- `API_SECRET_TOKEN` (**Required**): The shared secret for securing the WebSocket connection.
- `CLOUD_URL` (**Required**): The `wss://` URL of the deployed Cloud Dashboard.
- `NT_INCOMING` (Optional): Path to the NinjaTrader `incoming` folder. Defaults to `~/Documents/NinjaTrader 8/incoming`.

## Security Considerations
- **Use a strong, unique `API_SECRET_TOKEN`** for production deployments.
- **Always use `https` for the dashboard and `wss://` in the `CLOUD_URL`**. Cloud providers like Railway handle SSL automatically.
- Choose a strong `DASHBOARD_PASS` to protect access to the web interface.
- No sensitive data is ever stored in the cloud. The dashboard only displays real-time data and deletes it on refresh.