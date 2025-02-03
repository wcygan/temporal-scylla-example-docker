#!/bin/bash

# Start the workflow.
echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") - Starting workflow..."
start_response=$(grpcurl -plaintext -d '{
  "inputData": "foobar"
}' localhost:8081 workflow.v1.WorkflowService.StartWorkflow)

# Extract the workflow ID from the response.
workflowId=$(echo "$start_response" | jq -r '.workflowId')
echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") - Workflow started with ID: $workflowId"

# Poll the workflow status every 0.25 seconds.
status=""
running_seen=false
while true; do
  poll_response=$(grpcurl -plaintext -d "{\"workflowId\": \"$workflowId\"}" localhost:8081 workflow.v1.WorkflowService.GetWorkflowStatus)
  status=$(echo "$poll_response" | jq -r '.status')
  
  # Print status with different formatting based on state
  timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  if [ "$status" == "WORKFLOW_STATUS_RUNNING" ]; then
    running_seen=true
    echo -e "\033[33m$timestamp - Workflow status: $status\033[0m"  # Yellow for running
  elif [ "$status" == "WORKFLOW_STATUS_COMPLETED" ]; then
    echo -e "\033[32m$timestamp - Workflow status: $status\033[0m"  # Green for completed
    break
  else
    echo -e "\033[31m$timestamp - Workflow status: $status\033[0m"  # Red for other states
    break
  fi
  
  sleep 0.25
done

# Print the final result.
if [ "$status" == "WORKFLOW_STATUS_COMPLETED" ]; then
  result=$(echo "$poll_response" | jq -r '.result')
  echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") - Workflow completed successfully with result: $result"
  if [ "$running_seen" == "true" ]; then
    echo "✅ Successfully observed RUNNING state before completion"
  else
    echo "❌ Warning: Did not observe RUNNING state before completion"
  fi
else
  echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") - Workflow ended with status: $status"
fi
