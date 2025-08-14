package main

import (
	"bytes"
	"encoding/base64"
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

// Cluster matches the structure defined in the control-center.
type Cluster struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kubeconfig string `json:"kubeconfig"`
}

// Deployment matches the structure defined in the control-center.
type Deployment struct {
	ID        string    `json:"id"`
	ClusterID string    `json:"cluster_id"`
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
	case "clusters":
		handleClustersCmd(os.Args[2:])
	case "deploy":
		handleDeployCmd(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func handleClustersCmd(args []string) {
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		listClusters()
	case "add":
		addClusterCmd := flag.NewFlagSet("add", flag.ExitOnError)
		name := addClusterCmd.String("name", "", "The name of the cluster.")
		kubeconfigPath := addClusterCmd.String("kubeconfig", "", "Path to the kubeconfig file.")
		addClusterCmd.Parse(args[1:])

		if *name == "" || *kubeconfigPath == "" {
			fmt.Println("Error: --name and --kubeconfig flags are required for add command.")
			addClusterCmd.Usage()
			os.Exit(1)
		}
		addCluster(*name, *kubeconfigPath)
	default:
		fmt.Printf("Unknown subcommand for 'clusters': %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func handleDeployCmd(args []string) {
	deployCmd := flag.NewFlagSet("deploy", flag.ExitOnError)
	clusterID := deployCmd.String("cluster", "", "The ID of the cluster to deploy to.")
	imageURL := deployCmd.String("image", "", "The URL of the container image to deploy.")
	deployCmd.Parse(args)

	if *clusterID == "" || *imageURL == "" {
		fmt.Println("Error: --cluster and --image flags are required for deploy command.")
		deployCmd.Usage()
		os.Exit(1)
	}
	deployWorkload(*clusterID, *imageURL)
}

func printUsage() {
	fmt.Println("Usage: cctl <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  clusters list          List all registered clusters")
	fmt.Println("  clusters add           Register a new cluster")
	fmt.Println("  deploy                 Deploy a new workload to a cluster")
	fmt.Println("\nCluster Add arguments:")
	fmt.Println("  --name <name>          Name of the cluster")
	fmt.Println("  --kubeconfig <path>    Path to the kubeconfig file")
	fmt.Println("\nDeploy arguments:")
	fmt.Println("  --cluster <id>         ID of the cluster")
	fmt.Println("  --image <url>          URL of the container image")
}

func addCluster(name, kubeconfigPath string) {
	addr := os.Getenv("CONTROL_CENTER_ADDR")
	if addr == "" {
		addr = defaultControlCenterAddress
	}

	kubeconfigBytes, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		log.Fatalf("Failed to read kubeconfig file: %v", err)
	}
	kubeconfigB64 := base64.StdEncoding.EncodeToString(kubeconfigBytes)

	clusterData := map[string]string{
		"name":       name,
		"kubeconfig": kubeconfigB64,
	}
	jsonData, err := json.Marshal(clusterData)
	if err != nil {
		log.Fatalf("Failed to marshal cluster data: %v", err)
	}

	resp, err := http.Post(fmt.Sprintf("%s/api/v1/clusters", addr), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Failed to send cluster registration request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("Cluster registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	var cluster Cluster
	if err := json.NewDecoder(resp.Body).Decode(&cluster); err != nil {
		log.Fatalf("Failed to decode cluster response: %v", err)
	}

	fmt.Printf("Cluster '%s' registered successfully with ID: %s\n", cluster.Name, cluster.ID)
}

func deployWorkload(clusterID, imageURL string) {
	addr := os.Getenv("CONTROL_CENTER_ADDR")
	if addr == "" {
		addr = defaultControlCenterAddress
	}

	deployData := map[string]string{
		"cluster_id": clusterID,
		"image_url":  imageURL,
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
	fmt.Printf("  Cluster ID: %s\n", deployment.ClusterID)
	fmt.Printf("  Image: %s\n", deployment.ImageURL)
	fmt.Printf("  Status: %s\n", deployment.Status)
}

func listClusters() {
	addr := os.Getenv("CONTROL_CENTER_ADDR")
	if addr == "" {
		addr = defaultControlCenterAddress
	}

	resp, err := http.Get(fmt.Sprintf("%s/api/v1/clusters", addr))
	if err != nil {
		log.Fatalf("Fatal: Failed to connect to control center: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Error: Control center returned non-OK status: %s", resp.Status)
	}

	var clusters []*Cluster
	if err := json.NewDecoder(resp.Body).Decode(&clusters); err != nil {
		log.Fatalf("Fatal: Failed to decode response from control center: %v", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME")
	for _, cluster := range clusters {
		fmt.Fprintf(w, "%s\t%s\n",
			cluster.ID,
			cluster.Name,
		)
	}
	w.Flush()
}
