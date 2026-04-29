# Project Planning: Aegis Unified Manufacturing Correlation Engine

## 1. Mission and Vision
Project Aegis is designed to eliminate the data silos inherent in traditional ISA-95 manufacturing hierarchies. By directly correlating high-frequency, deterministic PLC telemetry (Level 1 & 2) with vehicle firmware versions and work orders managed by the MES (Level 4), Aegis enables unprecedented root-cause analysis and quality control during the vehicle production ramp.

## 2. Core Objectives
* **Silo Reduction:** Bypass the traditional SCADA bottleneck by streaming critical PLC data directly to a modern, cloud-native telemetry database.
* **Real-Time Correlation:** Automatically link physical assembly metrics (e.g., robotic arm torque, weld temperatures) to specific vehicle Identification Numbers (VINs) and software flashed at that station.
* **Predictive Quality:** Provide a user interface that allows floor managers to predict hardware defects based on minute anomalies in the PLC data streams.

## 3. Agile Implementation Phases

### Phase 1: Edge Integration & Ingestion (Levels 1-3)
* Develop a lightweight edge gateway service (Rust/C++) to interface with mock PLCs using industrial protocols like OPC UA or Modbus.
* Establish a high-throughput message broker (NATS or Kafka) to ingest the deterministic telemetry without dropping packets.

### Phase 2: MES Data Mocking & Relational Storage (Level 4)
* Build a mock MES microservice (Go) that generates production work orders, VINs, and firmware deployment logs.
* Store this transactional, relational data in a PostgreSQL database.

### Phase 3: The Correlation Engine
* Deploy a time-series/OLAP database (ClickHouse or TimescaleDB) to store the high-frequency PLC data.
* Write a stream-processing service that joins the live PLC data stream with the active MES work order, stamping every physical action with the corresponding vehicle VIN and firmware version.

### Phase 4: Manufacturing Intelligence Dashboard
* Develop a React/TypeScript frontend tailored for manufacturing supervisors.
* Build interactive visualizations that allow users to search a specific VIN and see the exact physical telemetry of the machines that built it, alongside the software it was running at the time.