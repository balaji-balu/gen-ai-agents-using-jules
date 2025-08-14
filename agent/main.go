package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	// Default control center address; can be overridden by the CONTROL_CENTER_ADDR environment variable.
	defaultControlCenterAddress = "http://localhost:8080"
)

// AgentInfo holds the ID assigned by the control center upon registration.
type AgentInfo struct {
	ID string `json:"id"`
}

// Deployment matches the structure in the control-center.
type Deployment struct {
	ID       string `json:"id"`
	AgentID  string `json:"agent_id"`
	ImageURL string `json:"image_url"`
	Status   string `json:"status"`
}

// RegistrationResponse is the expected response body from the registration endpoint.
type RegistrationResponse struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Status  string `json:"status"`
}

func main() {
	// Determine control center address from environment variable or use default.
	addr := os.Getenv("CONTROL_CENTER_ADDR")
	if addr == "" {
		addr = defaultControlCenterAddress
	}

	log.Printf("Agent starting, attempting to connect to control center at %s", addr)

	// 1. Register the agent with the control center.
	agentInfo, err := registerAgent(addr)
	if err != nil {
		log.Fatalf("Fatal: Failed to register agent: %v", err)
	}
	log.Printf("Agent registered successfully with ID: %s", agentInfo.ID)

	// 2. Start sending periodic heartbeats in a background goroutine.
	go sendHeartbeats(addr, agentInfo.ID)

	// 3. Start polling for new deployments.
	go pollForDeployments(addr, agentInfo.ID)

	// Keep the main application running indefinitely.
	log.Println("Agent is running. Press Ctrl+C to exit.")
	select {}
}

func pollForDeployments(addr, agentID string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	processedDeployments := make(map[string]bool)

	for {
		<-ticker.C
		log.Println("Polling for new deployments...")

		req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/deployments?agent_id=%s", addr, agentID), nil)
		if err != nil {
			log.Printf("Error creating deployment request: %v", err)
			continue
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error polling for deployments: %v", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("Polling for deployments failed with status %d", resp.StatusCode)
			resp.Body.Close()
			continue
		}

		var deployments []Deployment
		if err := json.NewDecoder(resp.Body).Decode(&deployments); err != nil {
			log.Printf("Error decoding deployment response: %v", err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		for _, dep := range deployments {
			// A simple mechanism to avoid re-processing deployments.
			if !processedDeployments[dep.ID] {
				log.Printf("Found new deployment %s for image %s", dep.ID, dep.ImageURL)
				handleDeployment(dep)
				processedDeployments[dep.ID] = true
			}
		}
	}
}

func handleDeployment(dep Deployment) {
	log.Printf("Handling deployment %s: Pulling image %s", dep.ID, dep.ImageURL)
	// In a future step, this will be replaced with actual containerd logic.
	log.Printf("Deployment %s handled (simulated).", dep.ID)
}


// registerAgent sends a POST request to the control center to register this agent.
func registerAgent(addr string) (*AgentInfo, error) {
	// In a real scenario, this address would be the agent's actual listening address.
	regData := map[string]string{"address": "agent-instance-1:9090"}
	jsonData, err := json.Marshal(regData)
	if err != nil {
		return nil, fmt.Errorf("could not marshal registration data: %w", err)
	}

	resp, err := http.Post(fmt.Sprintf("%s/api/v1/agents", addr), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("could not send registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	var regResponse RegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResponse); err != nil {
		return nil, fmt.Errorf("could not decode registration response: %w", err)
	}

	return &AgentInfo{ID: regResponse.ID}, nil
}

// sendHeartbeats periodically sends a POST request to the control center's heartbeat endpoint.
func sendHeartbeats(addr, agentID string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		log.Println("Sending heartbeat...")

		heartbeatData := map[string]string{"id": agentID}
		jsonData, err := json.Marshal(heartbeatData)
		if err != nil {
			log.Printf("Error: could not marshal heartbeat data: %v", err)
			continue
		}

		resp, err := http.Post(fmt.Sprintf("%s/api/v1/heartbeat", addr), "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("Error: could not send heartbeat: %v", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("Error: heartbeat failed with status %d: %s", resp.StatusCode, string(body))
		}
		resp.Body.Close()
	}
}
