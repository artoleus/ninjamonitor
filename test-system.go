// test-system.go - Simple test to verify components work together
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type TestSnapshot struct {
	Timestamp  time.Time `json:"timestamp"`
	Account    string    `json:"account"`
	Balance    float64   `json:"balance"`
	Realized   float64   `json:"realized"`
	Unrealized float64   `json:"unrealized"`
	Positions  []struct {
		Instrument     string  `json:"instrument"`
		MarketPosition string  `json:"marketPosition"`
		Quantity       int     `json:"quantity"`
		AveragePrice   float64 `json:"averagePrice"`
		Unrealized     float64 `json:"unrealized"`
		CurrentPrice   float64 `json:"currentPrice"`
	} `json:"positions"`
	WorkingOrders []interface{} `json:"workingOrders"`
}

func main() {
	fmt.Println("Testing NinjaMonitor System")
	fmt.Println("1. Make sure connection-server is running on :8080")
	fmt.Println("2. Make sure cloud-dashboard is running on :8081")
	fmt.Println("3. Sending test data...")

	// Create test snapshot
	snapshot := TestSnapshot{
		Timestamp:  time.Now(),
		Account:    "TestAccount123",
		Balance:    50000.00,
		Realized:   1250.75,
		Unrealized: -125.50,
		Positions: []struct {
			Instrument     string  `json:"instrument"`
			MarketPosition string  `json:"marketPosition"`
			Quantity       int     `json:"quantity"`
			AveragePrice   float64 `json:"averagePrice"`
			Unrealized     float64 `json:"unrealized"`
			CurrentPrice   float64 `json:"currentPrice"`
		}{
			{
				Instrument:     "ES 03-25",
				MarketPosition: "Long",
				Quantity:       2,
				AveragePrice:   5925.50,
				Unrealized:     -125.50,
				CurrentPrice:   5863.25,
			},
		},
		WorkingOrders: []interface{}{},
	}

	// Send to connection server
	data, _ := json.Marshal(snapshot)
	resp, err := http.Post("http://localhost:8080/webhook", "application/json", bytes.NewBuffer(data))
	if err != nil {
		fmt.Printf("Error sending to connection server: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Println("✓ Successfully sent test data to connection server")
		fmt.Println("✓ Check cloud dashboard at http://localhost:8081")
		fmt.Println("✓ Data should appear in the dashboard if WebSocket connection is working")
	} else {
		fmt.Printf("✗ Connection server returned status: %d\n", resp.StatusCode)
	}

	// Wait a moment then send a command test
	time.Sleep(2 * time.Second)
	fmt.Println("\n4. Testing command relay...")
	
	commandPayload := map[string]string{"account": "TestAccount123"}
	commandData, _ := json.Marshal(commandPayload)
	
	resp2, err := http.Post("http://localhost:8081/api/flatten_account", "application/json", bytes.NewBuffer(commandData))
	if err != nil {
		fmt.Printf("Error sending command to cloud dashboard: %v\n", err)
		return
	}
	defer resp2.Body.Close()

	if resp2.StatusCode == 200 {
		fmt.Println("✓ Successfully sent test command to cloud dashboard")
		fmt.Println("✓ Check connection server logs for command execution")
	} else {
		fmt.Printf("✗ Cloud dashboard returned status: %d\n", resp2.StatusCode)
	}

	fmt.Println("\nTest complete!")
}