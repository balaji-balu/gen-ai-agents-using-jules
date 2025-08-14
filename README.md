# Edge Orchestration System with Kubernetes

A simple edge orchestration system built with Go. This project provides a basic framework for managing and deploying containerized workloads to multiple Kubernetes clusters.

## Overview

The system has been refactored to use Kubernetes as its orchestration backbone, replacing the previous agent-based simulation model.

The system consists of two main components:
-   **Control Center:** A central server that manages a list of Kubernetes clusters and orchestrates deployments to them.
-   **cctl:** A command-line tool for interacting with the Control Center's API, allowing you to register clusters and deploy applications.

## Components

### 1. Control Center (`control-center`)

The Control Center is the brain of the operation. It's an HTTP server that exposes a RESTful API for managing clusters and deployments.

-   **Cluster Management:** Keeps track of all registered Kubernetes clusters. For simplicity, it stores their `kubeconfig` files in memory.
-   **Deployment Orchestration:** It receives deployment requests from `cctl` and creates `Deployment` resources in the target Kubernetes clusters using the official `client-go` library.
-   **In-Memory Storage:** For simplicity, the current implementation uses in-memory storage, meaning all data (clusters, deployments) is lost upon restart.

### 2. Control Center CLI (`cctl`)

`cctl` is a command-line interface that provides a human-friendly way to interact with the Control Center's API.

-   **Manage Clusters:** Register new Kubernetes clusters with the Control Center and list the ones already registered.
-   **Create Deployments:** Deploy a new container-based workload to a registered cluster.

## Getting Started

The easiest way to get the system up and running is with `docker-compose`.

### Prerequisites

-   A running Kubernetes cluster. For local development, you can use tools like [Kind](https://kind.sigs.k8s.io/), [k3s](https://k3s.io/), or [Minikube](https://minikube.sigs.k8s.io/docs/start/).
-   Your `kubeconfig` file for the cluster must be accessible.
-   Docker
-   Docker Compose
-   Go (for building and running the `cctl` tool)

### Running the System

1.  **Start the Control Center:**

    ```bash
    docker-compose up --build
    ```

    This command will build the Docker image for the `control-center` service and start it.

2.  **Build the `cctl` tool:**

    In a separate terminal, navigate to the `cctl` directory and build the binary:

    ```bash
    cd cctl
    go build .
    ```

## Usage

Once the services are running, you can use the `cctl` tool to interact with the system.

### 1. Register a Kubernetes Cluster

First, you need to register your Kubernetes cluster with the control center. You will need the path to your `kubeconfig` file.

```bash
# Replace "my-local-cluster" with a name of your choice
# and "~/.kube/config" with the actual path to your kubeconfig file.
./cctl clusters add --name "my-local-cluster" --kubeconfig ~/.kube/config
```

You should see a confirmation message with the new cluster's ID.

### 2. List Registered Clusters

You can verify that your cluster was registered by listing all clusters:

```bash
./cctl clusters list
```

You should see output similar to this:

```
ID                                      NAME
xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx    my-local-cluster
```

### 3. Deploy a Workload

Now, you can deploy a workload to the registered cluster. You'll need the cluster's ID from the previous step.

```bash
# Replace <CLUSTER_ID> with the actual ID of your cluster
./cctl deploy --cluster <CLUSTER_ID> --image "nginx:latest"
```

You'll see a confirmation that the deployment was created:

```
Deployment created successfully!
  ID: dep-xxxxxxxx
  Cluster ID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  Image: nginx:latest
  Status: pending
```

The control center will then create a `Deployment` in your Kubernetes cluster. You can verify this using `kubectl`:

```bash
kubectl get deployments
```

You should see a deployment named `dep-xxxxxxxx` running in the `default` namespace.

## API Endpoints

The `control-center` exposes the following API endpoints:

-   `POST /api/v1/clusters`: Register a new cluster.
-   `GET /api/v1/clusters`: List all registered clusters.
-   `POST /api/v1/deployments`: Create a new deployment in a registered cluster.
-   `GET /api/v1/deployments?cluster_id=<id>`: List deployments for a specific cluster.

## Roadmap
- openapi spec
- security
    - mTLS between cctl, control-center, and Kubernetes API servers
    - user auth and authorization (rbac/multitenancy), 
- harbor, local container registry
- Talos integration for programmatic cluster provisioning
