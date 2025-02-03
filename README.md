# Temporal ScyllaDB Example with Docker

## Quick Start

Start all services:

```bash
docker-compose up --build
```

The following services will be available:
- Temporal UI: http://localhost:8080 (view the workflow history)
- Workflow Server: http://localhost:8081 (start and check the workflow status)
- ScyllaDB: localhost:9042 (view the data in the database)
- Temporal: localhost:7233

## Testing the Workflow Service

### Install Testing Tools

First, install the necessary tools:

```bash
brew install grpcurl
brew install grpcui
brew install cassandra
```

### Using `test.sh`

The script [test.sh](./test.sh) will start the workflow, poll the status, and print the final result.

```bash
./test.sh
2025-02-03T07:05:44Z - Starting workflow...
2025-02-03T07:05:44Z - Workflow started with ID: workflow-075a2cf6-19ca-4e2e-ac73-45576011fb72
2025-02-03T07:05:44Z - Workflow status: WORKFLOW_STATUS_RUNNING
2025-02-03T07:05:44Z - Workflow status: WORKFLOW_STATUS_RUNNING
2025-02-03T07:05:45Z - Workflow status: WORKFLOW_STATUS_RUNNING
2025-02-03T07:05:45Z - Workflow status: WORKFLOW_STATUS_RUNNING
2025-02-03T07:05:45Z - Workflow status: WORKFLOW_STATUS_RUNNING
2025-02-03T07:05:46Z - Workflow status: WORKFLOW_STATUS_RUNNING
2025-02-03T07:05:46Z - Workflow status: WORKFLOW_STATUS_COMPLETED
2025-02-03T07:05:46Z - Workflow completed successfully with result: validated: foobar -> processed -> finalized
✅ Successfully observed RUNNING state before completion
```

### Using grpcurl

List available services:

```bash
grpcurl -plaintext localhost:8081 list
```

Start a workflow:

```bash
grpcurl -plaintext -d '{
  "inputData": "foobar"
}' localhost:8081 workflow.v1.WorkflowService.StartWorkflow
```

Check workflow status:

```bash
grpcurl -plaintext -d '{
  "workflowId": "workflow-6dad30bd-94e0-439a-8357-ebecb0dedcb2"
}' localhost:8081 workflow.v1.WorkflowService.GetWorkflowStatus
```

### Using grpcui

Launch the interactive web UI:

```bash
grpcui -plaintext localhost:8081
```

This will open a browser window where you can:
1. Browse available services
2. Execute RPCs interactively
3. View request/response messages
4. Test different input combinations

### Using cqlsh

```bash
docker exec -it temporal-scylladb cqlsh

DESCRIBE KEYSPACES;
USE temporal;
DESCRIBE TABLES;
SELECT * FROM executions LIMIT 10;

SELECT workflow_id,
       run_id,
       visibility_ts,
       execution_state,
       workflow_last_write_version
FROM executions
LIMIT 5;

SELECT shard_id,
       type,
       workflow_id,
       run_id,
       current_run_id,
       execution_state
FROM executions
WHERE shard_id = 1
LIMIT 5;

exit;
```