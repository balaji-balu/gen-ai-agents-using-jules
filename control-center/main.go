package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Deployment represents a workload to be deployed on an agent.
type Deployment struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	ImageURL  string    `json:"image_url"`
	Status    string    `json:"status"` // e.g., "pending", "running", "failed"
	CreatedAt time.Time `json:"created_at"`
}

// DeploymentRequest is the body for a POST /deployments request.
type DeploymentRequest struct {
	AgentID  string `json:"agent_id"`
	ImageURL string `json:"image_url"`
}

// DeploymentStore manages the collection of deployments.
type DeploymentStore struct {
	sync.Mutex
	deployments map[string]*Deployment
	byAgent     map[string][]*Deployment // Index for quick lookup by agent
}

// NewDeploymentStore creates a new in-memory deployment store.
func NewDeploymentStore() *DeploymentStore {
	return &DeploymentStore{
		deployments: make(map[string]*Deployment),
		byAgent:     make(map[string][]*Deployment),
	}
}

// Create creates a new deployment and stores it.
func (s *DeploymentStore) Create(agentID, imageURL string) *Deployment {
	s.Lock()
	defer s.Unlock()

	dep := &Deployment{
		ID:        fmt.Sprintf("dep-%s", uuid.New().String()[:8]),
		AgentID:   agentID,
		ImageURL:  imageURL,
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}
	s.deployments[dep.ID] = dep
	s.byAgent[agentID] = append(s.byAgent[agentID], dep)

	log.Printf("Deployment %s created for agent %s with image %s", dep.ID, agentID, imageURL)
	return dep
}

// ListForAgent returns all deployments for a given agent.
func (s *DeploymentStore) ListForAgent(agentID string) []*Deployment {
	s.Lock()
	defer s.Unlock()
	// A copy is returned to avoid race conditions on the slice itself
	deps := make([]*Deployment, len(s.byAgent[agentID]))
	copy(deps, s.byAgent[agentID])
	return deps
}

// Agent represents an edge agent connected to the control center.
type Agent struct {
	ID       string    `json:"id"`
	Address  string    `json:"address"`
	LastSeen time.Time `json:"last_seen"`
	Status   string    `json:"status"`
}

// AgentStore manages the collection of registered agents.
type AgentStore struct {
	sync.Mutex
	agents map[string]*Agent
}

// NewAgentStore creates a new in-memory agent store.
func NewAgentStore() *AgentStore {
	return &AgentStore{
		agents: make(map[string]*Agent),
	}
}

// Register creates a new agent, assigns it an ID, and stores it.
func (s *AgentStore) Register(addr string) *Agent {
	s.Lock()
	defer s.Unlock()

	id := uuid.New().String()
	agent := &Agent{
		ID:       id,
		Address:  addr,
		LastSeen: time.Now().UTC(),
		Status:   "online",
	}
	s.agents[id] = agent
	log.Printf("Agent registered: %s at %s", id, addr)
	return agent
}

// Heartbeat updates an agent's last seen time.
func (s *AgentStore) Heartbeat(id string) bool {
	s.Lock()
	defer s.Unlock()

	agent, exists := s.agents[id]
	if !exists {
		return false
	}
	agent.LastSeen = time.Now().UTC()
	agent.Status = "online"
	log.Printf("Heartbeat from agent: %s", id)
	return true
}

// List returns all registered agents, updating their status if they've missed heartbeats.
func (s *AgentStore) List() []*Agent {
	s.Lock()
	defer s.Unlock()

	// Update status based on last seen time before listing.
	// An agent is considered offline if it hasn't sent a heartbeat in over 45 seconds.
	for _, agent := range s.agents {
		if time.Since(agent.LastSeen) > 45*time.Second {
			agent.Status = "offline"
		}
	}

	list := make([]*Agent, 0, len(s.agents))
	for _, agent := range s.agents {
		list = append(list, agent)
	}
	return list
}

// RegisterRequest defines the body for the agent registration request.
type RegisterRequest struct {
	Address string `json:"address"`
}

// HeartbeatRequest defines the body for the agent heartbeat request.
type HeartbeatRequest struct {
	ID string `json:"id"`
}

func main() {
	agentStore := NewAgentStore()
	deploymentStore := NewDeploymentStore()

	http.HandleFunc("/api/v1/deployments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			agentID := r.URL.Query().Get("agent_id")
			if agentID == "" {
				http.Error(w, "agent_id query parameter is required", http.StatusBadRequest)
				return
			}
			deps := deploymentStore.ListForAgent(agentID)
			json.NewEncoder(w).Encode(deps)
		case http.MethodPost:
			var req DeploymentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			if req.AgentID == "" || req.ImageURL == "" {
				http.Error(w, "agent_id and image_url are required", http.StatusBadRequest)
				return
			}
			// TODO: Check if agent exists before creating deployment.
			dep := deploymentStore.Create(req.AgentID, req.ImageURL)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(dep)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Handler for /api/v1/agents
	// GET: List agents
	// POST: Register a new agent
	http.HandleFunc("/api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			agents := agentStore.List()
			json.NewEncoder(w).Encode(agents)
		case http.MethodPost:
			var req RegisterRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			if req.Address == "" {
				http.Error(w, "Address is required", http.StatusBadRequest)
				return
			}
			agent := agentStore.Register(req.Address)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(agent)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Handler for /api/v1/heartbeat
	// POST: Receives a heartbeat from a registered agent
	http.HandleFunc("/api/v1/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req HeartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if !agentStore.Heartbeat(req.ID) {
			http.Error(w, "Agent not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	log.Println("Control Center API server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
