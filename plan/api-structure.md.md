# API Architecture: The Polyglot Gateway Pattern

## 1. Architectural Philosophy
In the Aegis/Foresight ecosystem, no single API protocol is optimal for every workload. To achieve Staff-level performance, scalability, and developer experience, this project implements an API Gateway Pattern utilizing REST, GraphQL, and gRPC. Each protocol is strictly scoped to its domain-specific strengths.

## 2. External Communication (Client-to-Server)
The system exposes a unified Go-based API Gateway to external clients (React Web UI, external factory webhooks). 

* **GraphQL (The Primary UI Interface):**
  * **Role:** Serves as the data fetching layer for the React dashboard.
  * **Why:** Factory supervisors need varying levels of detail depending on the view. GraphQL prevents over-fetching by allowing the UI to query exact fields (e.g., only fetching `torque` and `timestamp`, ignoring `firmware_version`).
  * **Streaming:** Utilizes GraphQL Subscriptions over WebSockets to push live anomaly alerts to the frontend in real-time.
* **REST (The Utility Interface):**
  * **Role:** Handles system-level transactions that don't require complex data graphs.
  * **Why:** Used for user Authentication/Identity Management (OAuth/JWT) and ingesting slow-moving JSON webhooks from legacy Level 4 MES factory software.

## 3. Internal Communication (Server-to-Server)
Inside the cloud perimeter, microservices do not communicate via JSON. They use gRPC.

* **gRPC (The Internal Backbone):**
  * **Role:** High-speed, low-latency communication between the Go API Gateway and the internal Python/Go ML Inference workers.
  * **Why:** When the Gateway needs to request an anomaly score for 1,000 PLC metrics, passing this as a JSON array over HTTP is highly inefficient. gRPC uses Protocol Buffers (Protobuf), which serializes the data into lightweight binaries, drastically reducing network I/O and CPU parsing overhead.

## 4. Protobuf Contract Example
The contract between the Go Gateway and the ML Inference service is strictly typed using a `.proto` file. This ensures both services agree on the exact shape of the telemetry data, preventing runtime crashes.

```protobuf
syntax = "proto3";

package inference;
option go_package = "internal/pb";

// The gRPC Service Definition
service AnomalyScorer {
  // Accepts a batch of PLC telemetry and returns anomaly scores
  rpc ScoreTelemetryBatch (TelemetryBatch) returns (ScoreResponse) {}
}

// The binary data structures
message TelemetryBatch {
  string vin = 1;
  repeated float torque_readings = 2;
  repeated float temp_readings = 3;
}

message ScoreResponse {
  bool is_anomalous = 1;
  float confidence_score = 2;
  string error_context = 3;
}