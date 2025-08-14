package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Deployment represents a workload to be deployed on an agent.
type Deployment struct {
	ID        string    `json:"id"`
	ClusterID string    `json:"cluster_id"`
	ImageURL  string    `json:"image_url"`
	Status    string    `json:"status"` // e.g., "pending", "running", "failed"
	CreatedAt time.Time `json:"created_at"`
}

// DeploymentRequest is the body for a POST /deployments request.
type DeploymentRequest struct {
	ClusterID string `json:"cluster_id"`
	ImageURL  string `json:"image_url"`
}

// DeploymentStore manages the collection of deployments.
type DeploymentStore struct {
	sync.Mutex
	deployments map[string]*Deployment
	byCluster   map[string][]*Deployment // Index for quick lookup by cluster
}

// NewDeploymentStore creates a new in-memory deployment store.
func NewDeploymentStore() *DeploymentStore {
	return &DeploymentStore{
		deployments: make(map[string]*Deployment),
		byCluster:   make(map[string][]*Deployment),
	}
}

// Create creates a new deployment and stores it.
func (s *DeploymentStore) Create(clusterID, imageURL string) *Deployment {
	s.Lock()
	defer s.Unlock()

	dep := &Deployment{
		ID:        fmt.Sprintf("dep-%s", uuid.New().String()[:8]),
		ClusterID: clusterID,
		ImageURL:  imageURL,
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}
	s.deployments[dep.ID] = dep
	s.byCluster[clusterID] = append(s.byCluster[clusterID], dep)

	log.Printf("Deployment %s created for cluster %s with image %s", dep.ID, clusterID, imageURL)
	return dep
}

// ListForCluster returns all deployments for a given cluster.
func (s *DeploymentStore) ListForCluster(clusterID string) []*Deployment {
	s.Lock()
	defer s.Unlock()
	deps := make([]*Deployment, len(s.byCluster[clusterID]))
	copy(deps, s.byCluster[clusterID])
	return deps
}

// Cluster represents a Kubernetes cluster that can be a deployment target.
type Cluster struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kubeconfig string `json:"kubeconfig"` // Base64 encoded kubeconfig
}

// ClusterStore manages the collection of registered clusters.
type ClusterStore struct {
	sync.Mutex
	clusters map[string]*Cluster
}

// NewClusterStore creates a new in-memory cluster store.
func NewClusterStore() *ClusterStore {
	return &ClusterStore{
		clusters: make(map[string]*Cluster),
	}
}

// Add creates a new cluster, assigns it an ID, and stores it.
func (s *ClusterStore) Add(name, kubeconfig string) *Cluster {
	s.Lock()
	defer s.Unlock()

	id := uuid.New().String()
	cluster := &Cluster{
		ID:         id,
		Name:       name,
		Kubeconfig: kubeconfig,
	}
	s.clusters[id] = cluster
	log.Printf("Cluster registered: %s (ID: %s)", name, id)
	return cluster
}

// Get returns a cluster by its ID.
func (s *ClusterStore) Get(id string) (*Cluster, bool) {
	s.Lock()
	defer s.Unlock()
	cluster, exists := s.clusters[id]
	return cluster, exists
}

// List returns all registered clusters.
func (s *ClusterStore) List() []*Cluster {
	s.Lock()
	defer s.Unlock()
	list := make([]*Cluster, 0, len(s.clusters))
	for _, cluster := range s.clusters {
		list = append(list, cluster)
	}
	return list
}

// AddClusterRequest defines the body for the cluster registration request.
type AddClusterRequest struct {
	Name       string `json:"name"`
	Kubeconfig string `json:"kubeconfig"` // Base64 encoded
}

// deployToK8s creates a Kubernetes deployment in the target cluster.
func deployToK8s(cluster *Cluster, deployment *Deployment) error {
	// 1. Decode kubeconfig
	kubeconfigBytes, err := base64.StdEncoding.DecodeString(cluster.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to decode kubeconfig: %w", err)
	}

	// 2. Create client-go config
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigBytes)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	// 3. Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// 4. Create a deployment object
	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)
	k8sDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deployment.ID,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deployment.ID,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": deployment.ID,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "workload",
							Image: deployment.ImageURL,
						},
					},
				},
			},
		},
	}

	// 5. Create the deployment in the cluster
	log.Printf("Creating Kubernetes deployment %s with image %s in cluster %s...", deployment.ID, deployment.ImageURL, cluster.Name)
	_, err = deploymentsClient.Create(context.TODO(), k8sDeployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes deployment: %w", err)
	}

	log.Printf("Successfully created Kubernetes deployment %s", deployment.ID)
	return nil
}

func int32Ptr(i int32) *int32 { return &i }

func main() {
	clusterStore := NewClusterStore()
	deploymentStore := NewDeploymentStore()

	http.HandleFunc("/api/v1/deployments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			clusterID := r.URL.Query().Get("cluster_id")
			if clusterID == "" {
				http.Error(w, "cluster_id query parameter is required", http.StatusBadRequest)
				return
			}
			deps := deploymentStore.ListForCluster(clusterID)
			json.NewEncoder(w).Encode(deps)
		case http.MethodPost:
			var req DeploymentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			if req.ClusterID == "" || req.ImageURL == "" {
				http.Error(w, "cluster_id and image_url are required", http.StatusBadRequest)
				return
			}

			cluster, exists := clusterStore.Get(req.ClusterID)
			if !exists {
				http.Error(w, "Cluster not found", http.StatusNotFound)
				return
			}

			dep := deploymentStore.Create(req.ClusterID, req.ImageURL)

			// Asynchronously deploy to Kubernetes
			go func() {
				if err := deployToK8s(cluster, dep); err != nil {
					log.Printf("ERROR: Kubernetes deployment failed for %s: %v", dep.ID, err)
					// Here you might want to update the deployment status to "failed"
				}
			}()

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(dep)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Handler for /api/v1/clusters
	// GET: List clusters
	// POST: Register a new cluster
	http.HandleFunc("/api/v1/clusters", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			clusters := clusterStore.List()
			json.NewEncoder(w).Encode(clusters)
		case http.MethodPost:
			var req AddClusterRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			if req.Name == "" || req.Kubeconfig == "" {
				http.Error(w, "Name and kubeconfig are required", http.StatusBadRequest)
				return
			}
			cluster := clusterStore.Add(req.Name, req.Kubeconfig)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(cluster)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Println("Control Center API server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
