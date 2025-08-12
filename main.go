// main.go
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
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

var (
	mu        sync.RWMutex
	latest    = map[string]Snapshot{}
	clientsMu sync.Mutex
	clients   = map[chan []byte]bool{}
)

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
        <h3>NinjaTrader Remote Dashboard</h3>
        <div class="form-check form-switch"><input class="form-check-input" type="checkbox" role="switch" id="darkModeToggle"><label class="form-check-label" for="darkModeToggle">Dark Mode</label></div>
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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	incoming := os.Getenv("NT_INCOMING")
	if incoming == "" {
		home, _ := os.UserHomeDir()
		incoming = filepath.Join(home, "Documents", "NinjaTrader 8", "incoming")
	}
	if _, err := os.Stat(incoming); os.IsNotExist(err) {
		log.Printf("Warning: NinjaTrader incoming folder does not exist at %s", incoming)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { tpl.Execute(w, nil) })
	http.HandleFunc("/events", eventsHandler)
	http.HandleFunc("/webhook", webhookHandler)
	http.HandleFunc("/api/flatten", commandHandler("FLATTENEVERYTHING;;;;;;;;;;;;"))
	http.HandleFunc("/api/flatten_account", accountCommandHandler("FLATTENEVERYTHING;ACCOUNT=%s;;;;;;;;;;;"))
	http.HandleFunc("/api/close_position", instrumentCommandHandler("CLOSEPOSITION;ACCOUNT=%s;INSTRUMENT=%s;;;;;;;;;;"))
	http.HandleFunc("/api/cancel_order", orderCommandHandler("CANCEL;ACCOUNT=%s;ORDERID=%s;;;;;;;;;;"))

	log.Printf("Starting server on :%s", port)
	log.Printf("Dashboard: http://localhost:%s", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	var snap Snapshot
	if err := json.Unmarshal(body, &snap); err != nil || snap.Account == "" {
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}

	// CORRECTED: This fixes the race condition by holding the lock during both write and read operations.
	mu.Lock()
	latest[snap.Account] = snap
	b, err := json.Marshal(latest)
	mu.Unlock()

	if err != nil {
		log.Printf("JSON Marshal error: %v", err)
		w.WriteHeader(http.StatusOK) // Still acknowledge webhook even if we can't broadcast
		return
	}

	broadcast(b)
	w.WriteHeader(http.StatusOK)
}

func commandHandler(line string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := writeOIF(os.Getenv("NT_INCOMING"), line); err != nil {
			http.Error(w, "write failed: "+err.Error(), http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

func accountCommandHandler(format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Account string `json:"account"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		line := fmt.Sprintf(format, p.Account)
		if err := writeOIF(os.Getenv("NT_INCOMING"), line); err != nil {
			http.Error(w, "write failed: "+err.Error(), http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

func instrumentCommandHandler(format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Account    string `json:"account"`
			Instrument string `json:"instrument"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		line := fmt.Sprintf(format, p.Account, p.Instrument)
		if err := writeOIF(os.Getenv("NT_INCOMING"), line); err != nil {
			http.Error(w, "write failed: "+err.Error(), http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

func orderCommandHandler(format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Account string `json:"account"`
			OrderId string `json:"orderId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		line := fmt.Sprintf(format, p.Account, p.OrderId)
		if err := writeOIF(os.Getenv("NT_INCOMING"), line); err != nil {
			http.Error(w, "write failed: "+err.Error(), http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

func writeOIF(incomingDir, line string) error {
	if incomingDir == "" {
		home, _ := os.UserHomeDir()
		incomingDir = filepath.Join(home, "Documents", "NinjaTrader 8", "incoming")
	}
	filename := fmt.Sprintf("oif_%d_%d.txt", time.Now().UnixNano(), rand.Intn(10000))
	path := filepath.Join(incomingDir, filename)
	return os.WriteFile(path, []byte(line+"\r\n"), 0644)
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan []byte, 10)
	clientsMu.Lock()
	clients[ch] = true
	clientsMu.Unlock()
	defer func() {
		clientsMu.Lock()
		delete(clients, ch)
		close(ch)
		clientsMu.Unlock()
	}()

	mu.RLock()
	b, _ := json.Marshal(latest)
	mu.RUnlock()
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

func broadcast(b []byte) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for ch := range clients {
		select {
		case ch <- b:
		default:
		}
	}
}
