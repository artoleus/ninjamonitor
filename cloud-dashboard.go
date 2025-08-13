// cloud-dashboard.go
package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Position struct {
	Instrument     string  `json:"instrument"`
	Symbol         string  `json:"symbol,omitempty"`
	MarketPosition string  `json:"marketPosition"`
	Quantity       int     `json:"quantity"`
	AveragePrice   float64 `json:"averagePrice"`
	Unrealized     float64 `json:"unrealized"`
	CurrentPrice   float64 `json:"currentPrice"`
}

type WorkingOrder struct {
	OrderId        string  `json:"orderId"`
	Instrument     string  `json:"instrument"`
	OrderType      string  `json:"orderType"`
	OrderAction    string  `json:"orderAction"`
	Quantity       int     `json:"quantity"`
	Filled         int     `json:"filled"`
	LimitPrice     float64 `json:"limitPrice"`
	StopPrice      float64 `json:"stopPrice"`
	State          string  `json:"state"`
	Name           string  `json:"name"`
	Oco            string  `json:"oco"`
	IsStopLoss     bool    `json:"isStopLoss"`
	IsProfitTarget bool    `json:"isProfitTarget"`
}

type Snapshot struct {
	Timestamp     time.Time      `json:"timestamp"`
	Account       string         `json:"account"`
	Balance       float64        `json:"balance"`
	Realized      float64        `json:"realized"`
	Unrealized    float64        `json:"unrealized"`
	Positions     []Position     `json:"positions"`
	WorkingOrders []WorkingOrder `json:"workingOrders"`
}

type Command struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
	ID      string                 `json:"id"`
}

type CloudDashboard struct {
	mu              sync.RWMutex
	latest          map[string]Snapshot
	webClientsMu    sync.Mutex
	webClients      map[chan []byte]bool
	connectionsMu   sync.Mutex
	connections     map[*websocket.Conn]*ConnectionClient
	upgrader        websocket.Upgrader
	// Authentication
	dashboardUser   string
	dashboardPass   string
	apiSecretToken  string
	sessions        map[string]time.Time
	sessionsMu      sync.Mutex
}

type ConnectionClient struct {
	conn        *websocket.Conn
	commandChan chan Command
	id          string
}

type WebSocketMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
	ID   string      `json:"id,omitempty"`
}

var loginTpl = template.Must(template.New("login").Parse(`
<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>NinjaTrader Login</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.2/dist/css/bootstrap.min.css" rel="stylesheet">
<style>
body { background: #f8f9fa; }
.login-container { max-width: 400px; margin: 100px auto; }
.card { box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
</style>
</head>
<body>
<div class="login-container">
    <div class="card">
        <div class="card-body">
            <h4 class="card-title text-center mb-4">NinjaTrader Dashboard</h4>
            {{if .Error}}<div class="alert alert-danger">{{.Error}}</div>{{end}}
            <form method="POST" action="/login">
                <div class="mb-3">
                    <label for="username" class="form-label">Username</label>
                    <input type="text" class="form-control" id="username" name="username" required>
                </div>
                <div class="mb-3">
                    <label for="password" class="form-label">Password</label>
                    <input type="password" class="form-control" id="password" name="password" required>
                </div>
                <button type="submit" class="btn btn-primary w-100">Login</button>
            </form>
        </div>
    </div>
</div>
</body>
</html>
`))

var tpl = template.Must(template.New("dash").Parse(`
<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>NT8 Remote Dashboard</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.2/dist/css/bootstrap.min.css" rel="stylesheet">
<link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.3/font/bootstrap-icons.min.css" rel="stylesheet">
<style>
/* Base Styles */
body { transition: background-color 0.3s, color 0.3s; }
.action-btn { cursor: pointer; }

/* --- Light Mode Styles --- */
body { background-color: #f8f9fa; }
.text-label { color: #6c757d !important; }
.text-normal { color: #212529 !important; }
.text-pnl-positive { color: #198754 !important; }
.text-pnl-negative { color: #dc3545 !important; }
.table-danger-custom { background-color: rgba(220, 53, 69, 0.15) !important; }
.table-success-custom { background-color: rgba(25, 135, 84, 0.15) !important; }

/* --- Dark Mode Styles --- */
body.dark-mode { background-color: #212529; color: #dee2e6; }
body.dark-mode .card { background-color: #343a40; border-color: #495057; }
body.dark-mode .card-title, body.dark-mode h3, body.dark-mode h6, body.dark-mode th { color: #f8f9fa; }
body.dark-mode .text-label { color: #adb5bd !important; }
body.dark-mode .text-normal, body.dark-mode td { color: #dee2e6 !important; }
body.dark-mode .text-pnl-positive { color: #52b788 !important; }
body.dark-mode .text-pnl-negative { color: #e57373 !important; }
body.dark-mode .table { border-color: #495057; }
body.dark-mode .table > :not(caption) > * > * { background-color: #343a40; border-color: #495057;}
body.dark-mode .table-hover > tbody > tr:hover > * { background-color: #495057; color: #f8f9fa; }
body.dark-mode .form-check-label { color: #dee2e6; }
body.dark-mode hr { border-top: 1px solid rgba(255, 255, 255, 0.1); }
body.dark-mode .text-muted { color: #6c757d !important; }
body.dark-mode .table-danger-custom { background-color: rgba(220, 53, 69, 0.3) !important; }
body.dark-mode .table-success-custom { background-color: rgba(25, 135, 84, 0.3) !important; }
</style>
</head>
<body>
<div class="container py-4">
    <div class="d-flex justify-content-between align-items-center">
        <h3>NinjaTrader Cloud Dashboard (Railway)</h3>
        <div class="d-flex align-items-center gap-3">
            <div class="form-check form-switch"><input class="form-check-input" type="checkbox" role="switch" id="darkModeToggle"><label class="form-check-label" for="darkModeToggle">Dark Mode</label></div>
            <a href="/logout" class="btn btn-outline-secondary btn-sm">Logout</a>
        </div>
    </div>
    <div id="accounts" class="mt-3"></div>
    <div class="mt-4"><button id="flattenAll" class="btn btn-danger" data-action="flatten-all">Emergency Flatten (ALL Accounts)</button></div>
    <p class="text-muted small mt-3">Connected: <span id="status">--</span> | Last Update: <span id="lastUpdate">--</span></p>
</div>

<script>
const darkModeToggle = document.getElementById('darkModeToggle');
const body = document.body;
function setDarkMode(isDark) {
    localStorage.setItem('theme', isDark ? 'dark' : 'light');
    body.classList.toggle('dark-mode', isDark);
    darkModeToggle.checked = isDark;
}
darkModeToggle.addEventListener('click', () => setDarkMode(darkModeToggle.checked));
setDarkMode(localStorage.getItem('theme') === 'dark');

let evt = new EventSource('/events');
evt.onopen = () => { document.getElementById('status').innerText = 'connected'; };
evt.onerror = () => { document.getElementById('status').innerText = 'disconnected'; };
evt.onmessage = function(e){
    document.getElementById('lastUpdate').innerText = new Date().toLocaleTimeString();
    try { render(JSON.parse(e.data)); } catch(err) { console.error('Parse error:', err); }
};

async function sendCommand(url, body) {
    const msg = body.account ? 'Action: ' + url + '\nDetails: ' + JSON.stringify(body) : 'FLATTEN EVERYTHING?';
    if (!confirm('Are you sure?\n\n' + msg)) return;
    try {
        const resp = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
        if (!resp.ok) alert('Failed to send command: ' + resp.statusText);
    } catch (err) { alert('Error sending command: ' + err.message); }
}

document.addEventListener('click', (e) => {
    const target = e.target.closest('[data-action]');
    if (!target) return;
    const { action, account, instrument, orderId } = target.dataset;
    switch(action) {
        case 'flatten-all': sendCommand('/api/flatten', {}); break;
        case 'flatten-account': sendCommand('/api/flatten_account', { account }); break;
        case 'close-position': sendCommand('/api/close_position', { account, instrument }); break;
        case 'cancel-order': sendCommand('/api/cancel_order', { account, orderId }); break;
    }
});
document.getElementById('flattenAll').dataset.action = 'flatten-all';

function render(data) {
    const container = document.getElementById('accounts');
    container.innerHTML = '';
    for (const acc of Object.keys(data).sort()) {
        const snap = data[acc];
        const card = document.createElement('div');
        card.className = 'card mb-3';
        
        let positionsTable = '<p class="text-label">No open positions.</p>';
        if (snap.positions && snap.positions.length > 0) {
            let rows = snap.positions.map(p => {
                const pnlClass = p.unrealized >= 0 ? 'text-pnl-positive' : 'text-pnl-negative';
                return '<tr>' +
                    '<td>' + p.instrument + '</td>' +
                    '<td>' + p.marketPosition + '</td>' +
                    '<td>' + p.quantity + '</td>' +
                    '<td>' + p.averagePrice.toFixed(2) + '</td>' +
                    '<td>' + (p.currentPrice > 0 ? p.currentPrice.toFixed(2) : '--') + '</td>' +
                    '<td><strong class="' + pnlClass + '">' + p.unrealized.toFixed(2) + '</strong></td>' +
                    '<td><i class="bi bi-x-circle-fill text-pnl-negative action-btn" title="Close Position" data-action="close-position" data-account="' + acc + '" data-instrument="' + p.instrument + '"></i></td>' +
                '</tr>';
            }).join('');
            positionsTable = '<h6>Positions</h6><table class="table table-sm table-hover">' +
                '<thead><tr><th>Instrument</th><th>MP</th><th>Qty</th><th>Avg Price</th><th>Price</th><th>Unrealized</th><th></th></tr></thead>' +
                '<tbody>' + rows + '</tbody></table>';
        }
        
        let ordersTable = '<p class="text-label">No working orders.</p>';
        if (snap.workingOrders && snap.workingOrders.length > 0) {
            let rows = snap.workingOrders.map(o => {
                let price = '';
                if (o.orderType === 'Limit' || o.isProfitTarget) price = o.limitPrice.toFixed(2);
                else if (o.orderType.includes('Stop') || o.isStopLoss) price = o.stopPrice.toFixed(2);
                const rowClass = o.isStopLoss ? 'table-danger-custom' : (o.isProfitTarget ? 'table-success-custom' : '');
                return '<tr class="' + rowClass + '">' +
                    '<td>' + o.instrument + '</td>' +
                    '<td>' + (o.name || o.orderType) + '</td>' +
                    '<td>' + o.orderAction + '</td>' +
                    '<td>' + o.quantity + '</td>' +
                    '<td>' + price + '</td>' +
                    '<td>' + o.state + '</td>' +
                    '<td><i class="bi bi-x-circle-fill text-pnl-negative action-btn" title="Cancel Order" data-action="cancel-order" data-account="' + acc + '" data-order-id="' + o.orderId + '"></i></td>' +
                '</tr>';
            }).join('');
            ordersTable = '<h6>Working Orders</h6><table class="table table-sm table-hover">' +
                '<thead><tr><th>Instrument</th><th>Type</th><th>Action</th><th>Qty</th><th>Price</th><th>State</th><th></th></tr></thead>' +
                '<tbody>' + rows + '</tbody></table>';
        }
        
        const unrealizedCls = snap.unrealized >= 0 ? 'text-pnl-positive' : 'text-pnl-negative';
        card.innerHTML = '<div class="card-body">' +
            '<div class="d-flex justify-content-between align-items-center mb-2">' +
                '<h5 class="card-title mb-0">' + acc + '</h5>' +
                '<button class="btn btn-sm btn-warning" data-action="flatten-account" data-account="' + acc + '">Flatten ' + acc + '</button>' +
            '</div>' +
            '<p class="card-text small">' +
                '<span class="text-label">Balance: </span><strong class="text-normal">' + snap.balance.toFixed(2) + '</strong> | ' +
                '<span class="text-label">Realized P/L: </span><strong class="text-normal">' + snap.realized.toFixed(2) + '</strong> | ' +
                '<span class="text-label">Unrealized P/L: </span><strong class="' + unrealizedCls + '">' + snap.unrealized.toFixed(2) + '</strong> | ' +
                '<span class="text-label">Updated: </span><span class="text-normal">' + new Date(snap.timestamp).toLocaleTimeString() + '</span>' +
            '</p><hr>' + positionsTable + ordersTable + '</div>';
        container.appendChild(card);
    }
}
</script>
</body>
</html>
`))

func NewCloudDashboard() *CloudDashboard {
	// Get auth credentials from environment
	dashUser := os.Getenv("DASHBOARD_USER")
	if dashUser == "" {
		dashUser = "admin"
	}
	dashPass := os.Getenv("DASHBOARD_PASS") 
	if dashPass == "" {
		dashPass = "ninja123" // Default password - change this!
	}
	apiSecretToken := os.Getenv("API_SECRET_TOKEN")
	if apiSecretToken == "" {
		apiSecretToken = generateRandomKey()
		log.Printf("Generated API Secret Token: %s", apiSecretToken)
		log.Printf("Set API_SECRET_TOKEN environment variable to this value for the connection-server.")
	}

	return &CloudDashboard{
		latest:        make(map[string]Snapshot),
		webClients:    make(map[chan []byte]bool),
		connections:   make(map[*websocket.Conn]*ConnectionClient),
		sessions:      make(map[string]time.Time),
		dashboardUser: dashUser,
		dashboardPass: dashPass,
		apiSecretToken: apiSecretToken,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
		},
	}
}

func generateRandomKey() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)
}

func (cd *CloudDashboard) Start() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	// Authentication routes
	http.HandleFunc("/login", cd.loginHandler)
	http.HandleFunc("/logout", cd.logoutHandler)
	
	// Protected routes
	http.HandleFunc("/", cd.requireAuth(func(w http.ResponseWriter, r *http.Request) { tpl.Execute(w, nil) }))
	http.HandleFunc("/events", cd.requireAuth(cd.eventsHandler))
	http.HandleFunc("/api/flatten", cd.requireAuth(cd.commandHandler("flatten_all")))
	http.HandleFunc("/api/flatten_account", cd.requireAuth(cd.accountCommandHandler("flatten_account")))
	http.HandleFunc("/api/close_position", cd.requireAuth(cd.instrumentCommandHandler("close_position")))
	http.HandleFunc("/api/cancel_order", cd.requireAuth(cd.orderCommandHandler("cancel_order")))
	
	// WebSocket with API key auth
	http.HandleFunc("/ws", cd.websocketHandler)

	log.Printf("Starting Cloud Dashboard on :%s", port)
	log.Printf("Dashboard: http://localhost:%s", port)
	log.Printf("Default login: %s / %s", cd.dashboardUser, cd.dashboardPass)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}

// Authentication middleware
func (cd *CloudDashboard) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionCookie, err := r.Cookie("session")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		cd.sessionsMu.Lock()
		sessionTime, exists := cd.sessions[sessionCookie.Value]
		cd.sessionsMu.Unlock()

		if !exists || time.Since(sessionTime) > 24*time.Hour {
			// Session expired
			cd.sessionsMu.Lock()
			delete(cd.sessions, sessionCookie.Value)
			cd.sessionsMu.Unlock()
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Update session time
		cd.sessionsMu.Lock()
		cd.sessions[sessionCookie.Value] = time.Now()
		cd.sessionsMu.Unlock()

		next(w, r)
	}
}

func (cd *CloudDashboard) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		loginTpl.Execute(w, nil)
		return
	}

	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Constant-time comparison to prevent timing attacks
		usernameOK := subtle.ConstantTimeCompare([]byte(username), []byte(cd.dashboardUser)) == 1
		passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(cd.dashboardPass)) == 1

		if usernameOK && passwordOK {
			// Create session
			sessionID := generateRandomKey()
			cd.sessionsMu.Lock()
			cd.sessions[sessionID] = time.Now()
			cd.sessionsMu.Unlock()

			// Set cookie
			http.SetCookie(w, &http.Cookie{
				Name:     "session",
				Value:    sessionID,
				Path:     "/",
				HttpOnly: true,
				Secure:   strings.HasPrefix(r.Header.Get("X-Forwarded-Proto"), "https"),
				SameSite: http.SameSiteLaxMode,
			})

			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Invalid credentials
		loginTpl.Execute(w, map[string]string{"Error": "Invalid username or password"})
		return
	}
}

func (cd *CloudDashboard) logoutHandler(w http.ResponseWriter, r *http.Request) {
	sessionCookie, err := r.Cookie("session")
	if err == nil {
		cd.sessionsMu.Lock()
		delete(cd.sessions, sessionCookie.Value)
		cd.sessionsMu.Unlock()
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-time.Hour),
		HttpOnly: true,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (cd *CloudDashboard) websocketHandler(w http.ResponseWriter, r *http.Request) {
	// Check API key for WebSocket connections (connection servers)
	authHeader := r.Header.Get("Authorization")
	expectedHeader := "Bearer " + cd.apiSecretToken
	if subtle.ConstantTimeCompare([]byte(authHeader), []byte(expectedHeader)) != 1 {
		// To prevent token leakage, log a generic unauthorized message.
		log.Printf("WebSocket unauthorized: Invalid token.")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := cd.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &ConnectionClient{
		conn:        conn,
		commandChan: make(chan Command, 10),
		id:          fmt.Sprintf("conn_%d", time.Now().UnixNano()),
	}

	cd.connectionsMu.Lock()
	cd.connections[conn] = client
	cd.connectionsMu.Unlock()

	log.Printf("Connection server connected: %s", client.id)

	// Send initial snapshot if available
	cd.mu.RLock()
	if len(cd.latest) > 0 {
		data, _ := json.Marshal(WebSocketMessage{
			Type: "snapshot",
			Data: cd.latest,
		})
		cd.mu.RUnlock()
		conn.WriteMessage(websocket.TextMessage, data)
	} else {
		cd.mu.RUnlock()
	}

	// Start goroutines for this connection
	go cd.handleConnection(client)
	go cd.sendCommands(client)
}

func (cd *CloudDashboard) handleConnection(client *ConnectionClient) {
	defer func() {
		cd.connectionsMu.Lock()
		delete(cd.connections, client.conn)
		cd.connectionsMu.Unlock()
		close(client.commandChan)
		client.conn.Close()
		log.Printf("Connection server disconnected: %s", client.id)
	}()

	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg WebSocketMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Invalid WebSocket message: %v", err)
			continue
		}

		switch msg.Type {
		case "snapshot":
			if dataMap, ok := msg.Data.(map[string]interface{}); ok {
				cd.mu.Lock()
				for account, snapData := range dataMap {
					if snapBytes, err := json.Marshal(snapData); err == nil {
						var snap Snapshot
						if err := json.Unmarshal(snapBytes, &snap); err == nil {
							cd.latest[account] = snap
						}
					}
				}
				broadcastData, _ := json.Marshal(cd.latest)
				cd.mu.Unlock()
				cd.broadcast(broadcastData)
			}
		case "command_ack":
			log.Printf("Command acknowledged: %s", msg.ID)
		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

func (cd *CloudDashboard) sendCommands(client *ConnectionClient) {
	for cmd := range client.commandChan {
		data, _ := json.Marshal(cmd)
		if err := client.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Failed to send command to connection server: %v", err)
			return
		}
	}
}

func (cd *CloudDashboard) commandHandler(cmdType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd := Command{
			Type:    cmdType,
			Payload: make(map[string]interface{}),
			ID:      fmt.Sprintf("cmd_%d", time.Now().UnixNano()),
		}
		
		cd.sendCommandToConnections(cmd)
		w.WriteHeader(http.StatusOK)
	}
}

func (cd *CloudDashboard) accountCommandHandler(cmdType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Account string `json:"account"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		cmd := Command{
			Type: cmdType,
			Payload: map[string]interface{}{
				"account": p.Account,
			},
			ID: fmt.Sprintf("cmd_%d", time.Now().UnixNano()),
		}

		cd.sendCommandToConnections(cmd)
		w.WriteHeader(http.StatusOK)
	}
}

func (cd *CloudDashboard) instrumentCommandHandler(cmdType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Account    string `json:"account"`
			Instrument string `json:"instrument"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		cmd := Command{
			Type: cmdType,
			Payload: map[string]interface{}{
				"account":    p.Account,
				"instrument": p.Instrument,
			},
			ID: fmt.Sprintf("cmd_%d", time.Now().UnixNano()),
		}

		cd.sendCommandToConnections(cmd)
		w.WriteHeader(http.StatusOK)
	}
}

func (cd *CloudDashboard) orderCommandHandler(cmdType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Account string `json:"account"`
			OrderId string `json:"orderId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		cmd := Command{
			Type: cmdType,
			Payload: map[string]interface{}{
				"account": p.Account,
				"orderId": p.OrderId,
			},
			ID: fmt.Sprintf("cmd_%d", time.Now().UnixNano()),
		}

		cd.sendCommandToConnections(cmd)
		w.WriteHeader(http.StatusOK)
	}
}

func (cd *CloudDashboard) sendCommandToConnections(cmd Command) {
	cd.connectionsMu.Lock()
	defer cd.connectionsMu.Unlock()

	for _, client := range cd.connections {
		select {
		case client.commandChan <- cmd:
		default:
			log.Printf("Command channel full for connection %s", client.id)
		}
	}
}

func (cd *CloudDashboard) eventsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan []byte, 10)
	
	cd.webClientsMu.Lock()
	cd.webClients[ch] = true
	cd.webClientsMu.Unlock()
	
	defer func() {
		cd.webClientsMu.Lock()
		delete(cd.webClients, ch)
		close(ch)
		cd.webClientsMu.Unlock()
	}()

	cd.mu.RLock()
	b, _ := json.Marshal(cd.latest)
	cd.mu.RUnlock()
	fmt.Fprintf(w, "data: %s\n\n", b)
	w.(http.Flusher).Flush()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-ticker.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (cd *CloudDashboard) broadcast(b []byte) {
	cd.webClientsMu.Lock()
	defer cd.webClientsMu.Unlock()
	for ch := range cd.webClients {
		select {
		case ch <- b:
		default:
		}
	}
}

func main() {
	dashboard := NewCloudDashboard()
	dashboard.Start()
}