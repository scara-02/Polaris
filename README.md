# Polaris: Real-Time IoT Fleet Orchestration Platform

Polaris is a distributed, event-driven microservices platform designed to ingest, process, and orchestrate high-frequency telemetry data from massive IoT fleets (drones, autonomous vehicles, logistics networks) in real time. 

Built as a high-performance alternative to traditional batch-processing fleet managers, Polaris leverages a custom in-memory spatial engine and stream-processing architecture to enable **autonomous,  fleet routing and predictive demand balancing**.

---

## The Core Concept

As logistics, ride-sharing, and drone delivery networks scale, they face a massive data bottleneck. Tens of thousands of vehicles continuously broadcast their GPS coordinates, speeds, and battery levels every second.

**The Polaris Solution:** Polaris flips this paradigm from *passive monitoring* to **active, real-time orchestration**. 
1. **Streaming-First:** Instead of writing directly to a database, Polaris catches data in a Redis Stream, acting as a massive shock absorber.
2. **Instant Spatial Awareness:** It feeds those coordinates into a custom, in-memory QuadTree (the "Brain"), bypassing database read-delays entirely. 
3. **Autonomous Action:** The system continuously evaluates the live map against predictive AI demand zones. When a geographic deficit is detected, Polaris doesn't wait for a human dispatcher; it instantly shoots a WebSocket `RELOCATE` directive back down to the idle physical hardware to pre-position the fleet.

Polaris isn't just a map showing where your fleet *is* it's an intelligent engine deciding where your fleet *needs to be*.

## ✨ Key Features

* **High-Throughput Ingestion Pipeline:** Utilizes **Redis Streams** as a "shock absorber" to decouple live WebSocket traffic from heavy database disk I/O.
* **Custom Spatial Engine:** A thread-safe, in-memory **QuadTree** algorithm written in Go enables lightning-fast geographical querying and exact Earth-curvature (Haversine) routing, bypassing slow database queries.
* **Event-Driven Microservices:** Completely decoupled architecture. The Edge Gateway handles WebSockets, while the Spatial Engine handles heavy compute. They communicate asynchronously via **Redis Pub/Sub**.
* **Autonomous Orchestration:** Evaluates live fleet density against spatial demand zones and dispatches physical `RELOCATE` directives directly to IoT hardware.
* **Predictive AI Clustering:** Uses SQL-based clustering on historical PostgreSQL data to predict future demand hotspots and pre-position fleet assets.
* **Live Command Center:** A **React/TypeScript** frontend featuring `Leaflet.js` map rendering, dynamic heatmaps, and `Chart.js` telemetry throughput monitoring.

---

## 🏗️ System Architecture

Polaris is split into three highly scalable layers:

1. **Edge Gateway :** A lightweight Go microservice that holds raw WebSocket connections, authenticates hardware, and pushes JSON payloads directly into Redis.
2. **Infrastructure :** * **Redis:** Acts as the message broker (Streams) and command bridge (Pub/Sub).
   * **PostgreSQL:** The permanent ledger for telemetry history and ML training data.
3. **Spatial Engine :** A heavy-compute Go microservice that pulls from Redis, maintains the live QuadTree state, archives batches to Postgres, runs the Orchestrator, and serves the REST API.
