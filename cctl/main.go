package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"text/tabwriter"
	"time"
)

const (
	// Default control center address; can be overridden by the CONTROL_CENTER_ADDR environment variable.
	defaultControlCenterAddress = "http://localhost:8080"
)

// Agent matches the structure defined in the control-center.
type Agent struct {
	ID       string    `json:"id"`
	Address  string    `json:"address"`
	LastSeen time.Time `json:"last_seen"`
	Status   string    `json:"status"`
}

// Deployment matches the structure defined in the control-center.
type Deployment struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	ImageURL  string    `json:"image_url"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "agents":
		handleAgentsCmd(os.Args[2:])
	case "deploy":
		handleDeployCmd(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func handleAgentsCmd(args []string) {
	if len(args) < 1 || args[0] != "list" {
		fmt.Println("Usage: cctl agents list")
		os.Exit(1)
	}
	listAgents()
}

func handleDeployCmd(args []string) {
	deployCmd := flag.NewFlagSet("deploy", flag.ExitOnError)
	agentID := deployCmd.String("agent", "", "The ID of the agent to deploy to.")
	imageURL := deployCmd.String("image", "", "The URL of the container image to deploy.")
	deployCmd.Parse(args)

	if *agentID == "" || *imageURL == "" {
		fmt.Println("Error: --agent and --image flags are required for deploy command.")
		deployCmd.Usage()
		os.Exit(1)
	}
	deployWorkload(*agentID, *imageURL)
}

func printUsage() {
	fmt.Println("Usage: cctl <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  agents list          List all registered agents")
	fmt.Println("  deploy               Deploy a new workload to an agent")
	fmt.Println("\nDeploy arguments:")
	fmt.Println("  --agent <id>         ID of the agent")
	fmt.Println("  --image <url>        URL of the container image")
}

func deployWorkload(agentID, imageURL string) {
	addr := os.Getenv("CONTROL_CENTER_ADDR")
	if addr == "" {
		addr = defaultControlCenterAddress
	}

	deployData := map[string]string{
		"agent_id":  agentID,
		"image_url": imageURL,
	}
	jsonData, err := json.Marshal(deployData)
	if err != nil {
		log.Fatalf("Failed to marshal deployment data: %v", err)
	}

	resp, err := http.Post(fmt.Sprintf("%s/api/v1/deployments", addr), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Failed to send deployment request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("Deployment request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var deployment Deployment
	if err := json.NewDecoder(resp.Body).Decode(&deployment); err != nil {
		log.Fatalf("Failed to decode deployment response: %v", err)
	}

	fmt.Printf("Deployment created successfully!\n")
	fmt.Printf("  ID: %s\n", deployment.ID)
	fmt.Printf("  Agent ID: %s\n", deployment.AgentID)
	fmt.Printf("  Image: %s\n", deployment.ImageURL)
	fmt.Printf("  Status: %s\n", deployment.Status)
}

// listAgents fetches the list of agents from the control center and prints them in a table.
func listAgents() {
	addr := os.Getenv("CONTROL_CENTER_ADDR")
	if addr == "" {
		addr = defaultControlCenterAddress
	}

	resp, err := http.Get(fmt.Sprintf("%s/api/v1/agents", addr))
	if err != nil {
		log.Fatalf("Fatal: Failed to connect to control center: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Error: Control center returned non-OK status: %s", resp.Status)
	}

	var agents []*Agent
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		log.Fatalf("Fatal: Failed to decode response from control center: %v", err)
	}

	// Use the standard library's tabwriter to format the output.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tADDRESS\tSTATUS\tLAST SEEN (UTC)")
	for _, agent := range agents {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			agent.ID,
			agent.Address,
			agent.Status,
			agent.LastSeen.Format(time.RFC3339),
		)
	}
	w.Flush()
}
