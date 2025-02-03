# Temporal ScyllaDB Example with Docker

```bash
docker-compose up
open http://localhost:8080
```

This guide provides a step-by-step walkthrough for setting up a proof of concept that integrates Temporal with ScyllaDB, using ConnectRPC and Go to implement a complex workflow system.

## Overview

The system demonstrates:
- ScyllaDB as the persistence layer for Temporal
- ConnectRPC for API communication
- A complex workflow implementation with four activities
- Docker-based deployment
- Buf for Protocol Buffer management

## 1. Infrastructure Setup with Docker

### ScyllaDB Container

Use the official ScyllaDB Docker image to start a ScyllaDB instance:

```yaml
scylladb:
  image: scylladb/scylla:6.2
  ports:
    - "9042:9042"
```

### Temporal Server Container

Configure Temporal to use ScyllaDB as its persistence layer:

```yaml
temporal:
  image: temporalio/auto-setup:1.26.2
  environment:
    - DB=cassandra
    - CASSANDRA_SEEDS=scylladb
  ports:
    - "7233:7233"   # Temporal frontend port
```

## 2. API Definition with Protocol Buffers

### Create Proto Schemas
1. Define your RPC service in proto files (e.g., `proto/workflow.proto`)
2. Include message definitions for data processing
3. Define service endpoints for workflow interaction

### Buf Integration
1. Install Buf and configure with `buf.yaml`
2. Generate language-specific stubs using `buf generate`
3. Use generated code in your Go ConnectRPC server

## 3. Go Service Implementation

### ConnectRPC Server
- Implement server using generated proto code
- Register handlers for RPC endpoints
- Handle workflow triggering requests

### Temporal Client Integration
- Initialize Temporal client in RPC handlers
- Execute workflows using Temporal's SDK
- Return workflow IDs or status in RPC responses

## 4. Workflow Implementation

### Workflow Structure
The example implements a four-activity workflow:

1. **Validate Input**: Data validation
2. **Transform Data**: Data transformation
3. **Enrich Data**: Data enrichment
4. **Format Output**: Final formatting

### Example Workflow Implementation

```go
func ComplexWorkflow(ctx workflow.Context, input string) (string, error) {
    var err error
    // Activity 1: Validate the input
    err = workflow.ExecuteActivity(ctx, ValidateInputActivity, input).Get(ctx, nil)
    if err != nil {
        return "", err
    }

    // Activity 2: Transform the data
    var transformed string
    err = workflow.ExecuteActivity(ctx, TransformActivity, input).Get(ctx, &transformed)
    if err != nil {
        return "", err
    }

    // Activity 3: Enrich the data
    var enriched string
    err = workflow.ExecuteActivity(ctx, EnrichActivity, transformed).Get(ctx, &enriched)
    if err != nil {
        return "", err
    }

    // Activity 4: Format the final output
    var output string
    err = workflow.ExecuteActivity(ctx, FormatOutputActivity, enriched).Get(ctx, &output)
    if err != nil {
        return "", err
    }
    return output, nil
}
```

## 5. Containerization

### Dockerfile Requirements
- Multi-stage build for Go binaries
- Worker and server packaging
- Alpine-based lightweight image
- Exposed ports for ConnectRPC server

### Docker Compose Configuration

```yaml
version: "3.8"
services:
  temporal:
    image: temporalio/auto-setup:latest
    environment:
      - DB=cassandra
      - CASSANDRA_SEEDS=scylladb
    ports:
      - "7233:7233"

  scylladb:
    image: scylladb/scylla:latest
    ports:
      - "9042:9042"

  myservice:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - temporal
      - scylladb
```

## 6. Running the Application

### Build and Deploy
```bash
docker-compose up --build
```

### Testing
1. Use HTTP clients (curl/Postman) to send requests to your ConnectRPC endpoint
2. Example endpoint: `http://localhost:8080/temporal.TemporalService/StartWorkflow`
3. Monitor logs from Temporal worker and server
4. Verify activity execution order and output

## Summary

This proof of concept demonstrates:
- Container orchestration with Docker Compose
- API definition and generation with Protocol Buffers and Buf
- Go service implementation with ConnectRPC
- Complex workflow implementation with Temporal
- Integration with ScyllaDB for persistence
- End-to-end testing and verification

The system provides a foundation for building scalable, reliable workflow-based applications using modern cloud-native technologies.