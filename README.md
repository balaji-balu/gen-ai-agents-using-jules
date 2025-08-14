# Edge Orchestration System

A simple edge orchestration system built with Go. This project provides a basic framework for managing and deploying workloads to a fleet of edge agents.

## Overview

The system consists of three main components:
-   **Control Center:** A central server that manages agents and orchestrates deployments.
-   **Agent:** A client that runs on edge devices, registers with the control center, and simulates workload deployment.
-   **cctl:** A command-line tool for interacting with the Control Center's API.

## Components

### 1. Control Center (`control-center`)

The Control Center is the brain of the operation. It's an HTTP server that exposes a RESTful API for managing agents and deployments.

-   **Agent Management:** Keeps track of all registered agents, their status (online/offline), and their last heartbeat.
-   **Deployment Orchestration:** Allows users to create new "deployments" (currently simulated) and assign them to specific agents.
-   **In-Memory Storage:** For simplicity, the current implementation uses in-memory storage, meaning all data (agents, deployments) is lost upon restart.

### 2. Agent (`agent`)

The Agent is a lightweight client designed to run on edge devices.

-   **Registration:** On startup, the agent registers itself with the Control Center to receive an ID.
-   **Heartbeats:** It periodically sends heartbeats to the Control Center to signal that it's still online.
-   **Deployment Polling:** It regularly polls the Control Center for new deployments assigned to it.
-   **Simulated Deployment:** When a new deployment is found, the agent logs a message to simulate the process of pulling and running a container image.

### 3. Control Center CLI (`cctl`)

`cctl` is a command-line interface that provides a human-friendly way to interact with the Control Center's API.

-   **List Agents:** View all agents that have registered with the Control Center.
-   **Create Deployments:** Deploy a new (simulated) workload to a registered agent.

## Getting Started

The easiest way to get the system up and running is with `docker-compose`.

### Prerequisites

-   Docker
-   Docker Compose
-   Go (for running the `cctl` tool)

### Running the System

1.  **Start the Control Center and an Agent:**

    ```bash
    docker-compose up --build
    ```

    This command will build the Docker images for the `control-center` and `agent` services and start them. You'll see logs from both services in your terminal.

2.  **Build the `cctl` tool:**

    In a separate terminal, navigate to the `cctl` directory and build the binary:

    ```bash
    cd cctl
    go build .
    ```

## Usage

Once the services are running, you can use the `cctl` tool to interact with the system.

### 1. List Registered Agents

After a few seconds, the agent should have registered with the control center. You can verify this by listing the agents:

```bash
./cctl agents list
```

You should see output similar to this:

```
ID                                      ADDRESS                 STATUS    LAST SEEN (UTC)
xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx    agent-instance-1:9090   online    YYYY-MM-DDTHH:MM:SSZ
```

*Note: The agent's ID is generated dynamically, so yours will be different.*

### 2. Deploy a Workload

Now, you can "deploy" a workload to the registered agent. You'll need the agent's ID from the previous step.

```bash
# Replace <AGENT_ID> with the actual ID of your agent
./cctl deploy --agent <AGENT_ID> --image "nginx:latest"
```

You'll see a confirmation that the deployment was created:

```
Deployment created successfully!
  ID: dep-xxxxxxxx
  Agent ID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  Image: nginx:latest
  Status: pending
```

If you watch the `docker-compose` logs, you will see a log message from the agent indicating that it has found and handled the new deployment.

## API Endpoints

The `control-center` exposes the following API endpoints:

-   `POST /api/v1/agents`: Register a new agent.
-   `GET /api/v1/agents`: List all registered agents.
-   `POST /api/v1/heartbeat`: Send a heartbeat from an agent.
-   `POST /api/v1/deployments`: Create a new deployment.
-   `GET /api/v1/deployments?agent_id=<id>`: List deployments for a specific agent.
