# MLOps Architecture: Aegis Foresight (Streaming Inference Engine)

## 1. System Overview
Foresight is an advanced Machine Learning Operations (MLOps) extension to the Aegis Correlation Engine. While Aegis handles the ingestion and correlation of factory telemetry, Foresight introduces real-time streaming inference to predict hardware defects before a vehicle leaves the assembly station.

This architecture intentionally decouples offline model training from online, low-latency stream scoring to ensure the factory floor is never bottlenecked by data science workloads.

## 2. The Offline Training Pipeline (Data Science)
Models are trained periodically (e.g., nightly) using historical data, ensuring they learn from the latest manufacturing anomalies.

* **Data Source:** A Python training job queries the ClickHouse OLAP database, pulling thousands of correlated robotic torque profiles and MES defect logs.
* **Algorithm:** An Unsupervised Anomaly Detection model (e.g., Isolation Forest or Autoencoder via `scikit-learn` or `PyTorch`).
* **Export Format:** To guarantee microsecond execution times in production, the trained model is exported into the **ONNX (Open Neural Network Exchange)** format.
* **Model Registry:** The `.onnx` file is versioned and uploaded to an AWS S3 bucket (or MLflow), acting as the central source of truth for production models.

## 3. The Online Inference Worker (Engineering)
This is a highly reliable microservice designed for extreme performance. It does not train models; it only executes them.

* **Technology:** Go (leveraging the `onnxruntime-go` wrapper) or a high-performance Python FastAPI worker.
* **Ingestion:** The worker subscribes to the active NATS JetStream topic containing the live PLC telemetry.
* **Execution:** As a PLC message arrives (e.g., `torque: 45.2Nm`), the worker feeds it into the cached ONNX model in memory.
* **The Output:** The model returns an `Anomaly_Score`. If the score exceeds a safety threshold (e.g., > 0.95), the worker instantly publishes a high-priority `defect_alert` back to NATS.

## 4. The Data Lifecycle & Feedback Loop
To ensure continuous improvement and "lessons learned," the system implements a closed-loop feedback mechanism:

1. **Prediction:** The Inference Worker flags a weld as anomalous.
2. **Action:** The React UI alerts the floor manager, who manually inspects the weld.
3. **Feedback:** The manager clicks "Confirm Defect" or "False Alarm" in the UI.
4. **Storage:** This human-verified label is saved back into ClickHouse via the GraphQL API.
5. **Retraining:** The next night, the Offline Training Pipeline uses this newly verified data to train a more accurate version of the ONNX model.

## 5. Deployment and Resilience
* **Hot-Swapping Models:** The Inference Worker periodically polls the S3 Model Registry. When a new ONNX model version is detected, it downloads the file and swaps it into memory using a Read/Write Mutex, ensuring zero downtime on the factory floor during model upgrades.
* **Failback:** If the ML model throws an execution error, the system is designed to "fail open"—it logs the error but allows the factory line to continue moving, ensuring experimental ML never stops production.