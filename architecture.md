# System Architecture: Aegis Correlation Engine

## 1. High-Level Concept: Flattening the ISA-95 Pyramid
Traditional factory software passes data sequentially up the hierarchy, causing delays and data loss. Project Aegis introduces a unified event stream that flattens this architecture. The Edge layer and the MES layer both publish to a central nervous system, allowing our correlation engine to marry physical metrics with software state.

## 2. The Edge/Control Layer (Level 1 & 2)
* **Components:** Mock PLCs, Robotic Arms, Conveyor Sensors.
* **Technology:** Rust-based Edge Gateway.
* **Functionality:** This layer generates highly deterministic telemetry. For example, a robotic arm reports its exact torque and position every 10 milliseconds during a fastening operation. The Rust gateway translates these industrial protocols (OPC UA) into lightweight JSON/Protobuf messages and publishes them to the message broker.

## 3. The MES / Enterprise Layer (Level 4)
* **Components:** Manufacturing Execution System, Firmware Deployment Manager.
* **Technology:** Go Microservice, PostgreSQL.
* **Functionality:** This layer acts as the "brain." When a vehicle chassis arrives at Station 5, the MES creates an active session for that VIN, noting that Firmware v2.1.4 is being flashed. It publishes this state change to the message broker.

## 4. The Streaming & Correlation Core
* **Components:** NATS JetStream (or Kafka), Go Correlation Worker, ClickHouse (OLAP).
* **Functionality:** This is the staff-level centerpiece. 
    1. The message broker receives the continuous stream of PLC torque data.
    2. It simultaneously holds the current MES state (e.g., "Station 5 is currently building VIN: 12345").
    3. The Go worker enriches the PLC data on the fly. It takes a raw PLC reading (`station_5_torque: 45Nm`) and appends the enterprise context (`vin: 12345`, `firmware: v2.1.4`) before saving it to ClickHouse.

## 5. The Presentation Layer (Manufacturing UI)
* **Components:** React.js, TypeScript, GraphQL API.
* **Functionality:** The frontend provides a reactive dashboard for quality assurance teams. Instead of looking at isolated PLC dashboards or isolated MES dashboards, a user can query: *"Show me the robotic arm torque variance for all vehicles flashed with Firmware v2.1.4 that later reported a steering defect."*