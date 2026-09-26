# AdTech MAB Engine - Intelligent Ad Serving Optimization System

An engineering thesis project implementing a distributed microservice architecture for intelligent online ad serving. The system leverages Reinforcement Learning algorithms (Multi-Armed Bandit) to dynamically maximize Click-Through Rate (CTR) in real time.

**Author:** Marcin Starzyk  

---

## 🏗 System Architecture

The system is containerized with Docker and consists of four collaborating microservices coordinated by a Caddy reverse proxy handling HTTPS termination:

1. **Backend API (Go / Chi):** The core service of the platform. Manages ad campaigns, records telemetry event logs (impressions/clicks), handles JWT authentication, and maintains persistent WebSocket connections with clients (Pub/Sub).
2. **AI MAB Engine (Python / gRPC):** A dedicated decision-making engine. Maintains Multi-Armed Bandit model weights in memory and evaluates inbound inference requests from Go to select the optimal ad creative.
3. **Admin Dashboard (React / Vite):** A single-page application (SPA) frontend used to configure campaigns, upload banner creatives, and inspect real-time performance analytics.
4. **Database (PostgreSQL):** Relational persistence layer storing the complete campaign schema, bcrypt-hashed credentials, and historical telemetry logs.

---

## 🚀 Key Engineering Highlights

- **Binary gRPC & Protobuf Communication:** Instead of text-heavy HTTP/JSON overhead, internal communication between Go and Python (`GetNextAd` decision requests and `RecordEvent` telemetry) runs over high-throughput, multiplexed gRPC frames.
- **State Hydration (Cold Start Resilience):** The Python ML engine holds model parameters in RAM. To prevent data loss across container restarts, the Go backend runs a `hydrateMABState` routine upon startup, querying historical events from PostgreSQL and syncing them to Python via gRPC (`SyncState`).
- **Concurrent WebSocket Broadcast (Pub/Sub):** Dashboard telemetry updates live. A custom middleware handles `http.Hijacker` interface delegation, allowing concurrent API access logging alongside safe, mutex-guarded broadcasts across multiple connected clients.
- **Soft Deletes (Archival Consistency):** Campaigns are retired without breaking data integrity (status set to `archived`), preventing orphaned click/impression records.
- **Stateless Security (JWT & bcrypt):** Passwords are salted and hashed using `bcrypt`. API authorization relies on stateless JSON Web Tokens (24h expiration window).

---

## 💻 Tech Stack

- **Backend:** Go 1.2x, Chi Router, `gorilla/websocket`, gRPC (Protocol Buffers)
- **ML Engine:** Python 3.1x, `grpcio`
- **Frontend:** React, Vite
- **Database:** PostgreSQL 15+
- **Infrastructure:** Docker, Docker Compose, Caddy Reverse Proxy
- **Load Testing:** Grafana k6

---

## ⚙️ Running Locally

The entire multi-service environment is orchestrated via Docker Compose, which automatically builds images and provisions networking.

### Prerequisites:
- `Docker` and `Docker Compose` installed.
- Free ports on the host machine: `8443` (HTTPS / Caddy), `8080` (API internal), `5432` (PostgreSQL).

### Quick Start:
1. Clone the repository:
   ```bash
   git clone https://github.com/madastt/ads-recommendation.git
   cd ads-recommendation
   ```
2. Build and run all services in detached mode:
   ```bash
   docker-compose up -d --build
   ```
3. The platform frontend and API are available via Caddy (make sure to accept the local self-signed certificates in your browser):
   - **Main Web Application:** `https://localhost:8443`

---

## 📚 API Documentation (Swagger / OpenAPI)

The Go backend features interactive OpenAPI documentation generated via Swag annotations.

- **Swagger UI:** `https://localhost:8443/swagger/index.html`

*To recompile documentation after updating Go endpoints:*
```bash
swag init -g cmd/api/main.go --parseInternal --parseDependency
docker-compose up -d --build api
```

---

## 📊 Performance & Load Testing

System throughput and latency were benchmarked under load using **k6**:
- The test suite simulates **50 concurrent virtual users** (~100 requests per second).
- Median decision latency (Go $\rightarrow$ gRPC $\rightarrow$ Python $\rightarrow$ Go response, excluding SSL handshake) runs at **~5 ms**.
- **99% of requests (p99)** resolve well under the standard Real-Time Bidding (RTB) deadline of **<100 ms**.
