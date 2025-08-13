// connection-server.go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

type ConnectionServer struct {
	mu              sync.RWMutex
	latest          map[string]Snapshot
	wsConn          *websocket.Conn
	wsConnMu        sync.Mutex
	reconnectChan   chan struct{}
	cloudURL        string
	apiSecretToken  string
	incomingDir     string
	commandChan     chan Command
	isConnected     bool
	connectAttempts int
	maxReconnects   int
}

func NewConnectionServer() *ConnectionServer {
	cloudURL := os.Getenv("CLOUD_URL")
	if cloudURL == "" {
		cloudURL = "ws://localhost:8081/ws"
	}

	apiSecretToken := os.Getenv("API_SECRET_TOKEN")
	if apiSecretToken == "" {
		log.Fatal("API_SECRET_TOKEN environment variable is required")
	}

	incoming := os.Getenv("NT_INCOMING")
	if incoming == "" {
		home, _ := os.UserHomeDir()
		incoming = filepath.Join(home, "Documents", "NinjaTrader 8", "incoming")
	}

	return &ConnectionServer{
		latest:        make(map[string]Snapshot),
		cloudURL:      cloudURL,
		apiSecretToken: apiSecretToken,
		incomingDir:   incoming,
		commandChan:   make(chan Command, 100), // Buffered channel to prevent blocking
		reconnectChan: make(chan struct{}, 1),
		maxReconnects: 10,
	}
}

func (cs *ConnectionServer) Start() {
	log.Printf("Starting Connection Server")
	log.Printf("Cloud URL: %s", cs.cloudURL)
	log.Printf("NT Incoming: %s", cs.incomingDir)

	if _, err := os.Stat(cs.incomingDir); os.IsNotExist(err) {
		log.Printf("Warning: NinjaTrader incoming folder does not exist at %s", cs.incomingDir)
	}

	// Start HTTP server for NinjaTrader webhooks
	http.HandleFunc("/webhook", cs.webhookHandler)
	
	go func() {
		log.Printf("Starting HTTP server on :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// Start WebSocket connection management
	go cs.manageWebSocket()
	
	// Start command processor
	go cs.processCommands()
	
	// Initial connection attempt
	cs.reconnectChan <- struct{}{}
	
	// Keep the main thread alive
	select {}
}

func (cs *ConnectionServer) webhookHandler(w http.ResponseWriter, r *http.Request) {
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

	// Apply same race condition fix as main.go:262-276
	cs.mu.Lock()
	cs.latest[snap.Account] = snap
	data, err := json.Marshal(map[string]interface{}{
		"type": "snapshot",
		"data": cs.latest,
	})
	cs.mu.Unlock()

	if err != nil {
		log.Printf("JSON Marshal error: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Send to cloud dashboard
	cs.sendToCloud(data)
	w.WriteHeader(http.StatusOK)
}

func (cs *ConnectionServer) sendToCloud(data []byte) {
	cs.wsConnMu.Lock()
	defer cs.wsConnMu.Unlock()

	if cs.wsConn == nil || !cs.isConnected {
		log.Printf("WebSocket not connected, dropping message")
		return
	}

	if err := cs.wsConn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Failed to send message to cloud: %v", err)
		cs.isConnected = false
		// Trigger reconnection
		select {
		case cs.reconnectChan <- struct{}{}:
		default:
		}
	}
}

func (cs *ConnectionServer) manageWebSocket() {
	for {
		select {
		case <-cs.reconnectChan:
			cs.connectToCloud()
		}
	}
}

func (cs *ConnectionServer) connectToCloud() {
	cs.wsConnMu.Lock()
	defer cs.wsConnMu.Unlock()

	if cs.wsConn != nil {
		cs.wsConn.Close()
		cs.wsConn = nil
		cs.isConnected = false
	}

	// Exponential backoff
	backoff := time.Duration(cs.connectAttempts) * time.Second
	if backoff > 60*time.Second {
		backoff = 60 * time.Second
	}

	if cs.connectAttempts > 0 {
		log.Printf("Reconnect attempt %d after %v", cs.connectAttempts, backoff)
		time.Sleep(backoff)
	}

	u, err := url.Parse(cs.cloudURL)
	if err != nil {
		log.Printf("Invalid cloud URL: %v", err)
		cs.scheduleReconnect()
		return
	}

	// Set API key header for authentication
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+cs.apiSecretToken)
	
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
	if err != nil {
		log.Printf("Failed to connect to cloud: %v", err)
		cs.scheduleReconnect()
		return
	}

	cs.wsConn = conn
	cs.isConnected = true
	cs.connectAttempts = 0
	log.Printf("Connected to cloud dashboard")

	// Start reading commands from cloud
	go cs.readFromCloud()

	// Send initial snapshot if we have data
	cs.mu.RLock()
	if len(cs.latest) > 0 {
		data, _ := json.Marshal(map[string]interface{}{
			"type": "snapshot",
			"data": cs.latest,
		})
		cs.mu.RUnlock()
		cs.wsConn.WriteMessage(websocket.TextMessage, data)
	} else {
		cs.mu.RUnlock()
	}
}

func (cs *ConnectionServer) scheduleReconnect() {
	cs.connectAttempts++
	if cs.connectAttempts > cs.maxReconnects {
		log.Printf("Max reconnection attempts reached, waiting 5 minutes")
		cs.connectAttempts = cs.maxReconnects // Cap the attempts
		time.Sleep(5 * time.Minute)
	}
	
	go func() {
		time.Sleep(1 * time.Second) // Prevent tight loop
		select {
		case cs.reconnectChan <- struct{}{}:
		default:
		}
	}()
}

func (cs *ConnectionServer) readFromCloud() {
	defer func() {
		cs.wsConnMu.Lock()
		cs.isConnected = false
		cs.wsConnMu.Unlock()
		cs.scheduleReconnect()
	}()

	for {
		_, message, err := cs.wsConn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			return
		}

		var cmd Command
		if err := json.Unmarshal(message, &cmd); err != nil {
			log.Printf("Invalid command received: %v", err)
			continue
		}

		// Queue command for processing
		select {
		case cs.commandChan <- cmd:
		default:
			log.Printf("Command queue full, dropping command: %s", cmd.Type)
		}
	}
}

func (cs *ConnectionServer) processCommands() {
	for cmd := range cs.commandChan {
		err := cs.executeCommand(cmd)
		
		// Send acknowledgment back to cloud
		ack := map[string]interface{}{
			"type": "command_ack",
			"id":   cmd.ID,
		}
		if err != nil {
			ack["error"] = err.Error()
			log.Printf("Command execution failed: %v", err)
		} else {
			ack["success"] = true
		}

		data, _ := json.Marshal(ack)
		cs.sendToCloud(data)
	}
}

func (cs *ConnectionServer) executeCommand(cmd Command) error {
	switch cmd.Type {
	case "flatten_all":
		return cs.writeOIF("FLATTENEVERYTHING;;;;;;;;;;;;")
	case "flatten_account":
		account := cmd.Payload["account"].(string)
		return cs.writeOIF(fmt.Sprintf("FLATTENEVERYTHING;ACCOUNT=%s;;;;;;;;;;;", account))
	case "close_position":
		account := cmd.Payload["account"].(string)
		instrument := cmd.Payload["instrument"].(string)
		return cs.writeOIF(fmt.Sprintf("CLOSEPOSITION;ACCOUNT=%s;INSTRUMENT=%s;;;;;;;;;;", account, instrument))
	case "cancel_order":
		account := cmd.Payload["account"].(string)
		orderID := cmd.Payload["orderId"].(string)
		return cs.writeOIF(fmt.Sprintf("CANCEL;ACCOUNT=%s;ORDERID=%s;;;;;;;;;;", account, orderID))
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

func (cs *ConnectionServer) writeOIF(line string) error {
	filename := fmt.Sprintf("oif_%d_%d.txt", time.Now().UnixNano(), rand.Intn(10000))
	path := filepath.Join(cs.incomingDir, filename)
	return os.WriteFile(path, []byte(line+"\r\n"), 0644)
}

func main() {
	server := NewConnectionServer()
	server.Start()
}