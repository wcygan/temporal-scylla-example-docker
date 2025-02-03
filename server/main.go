package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	// Replace with your generated package import path.
	"buf.build/gen/go/wcygan/temporal-scylla-example/connectrpc/go/workflow/v1/workflowv1connect"
	workflowv1pb "buf.build/gen/go/wcygan/temporal-scylla-example/protocolbuffers/go/workflow/v1"
)

// workflowServiceServer implements the WorkflowService defined in our proto.
type workflowServiceServer struct {
	workflowv1connect.UnimplementedWorkflowServiceHandler
}

// StartWorkflow starts a new Temporal workflow and returns its identifiers.
func (s *workflowServiceServer) StartWorkflow(ctx context.Context, req *workflowv1pb.StartWorkflowRequest) (*workflowv1pb.StartWorkflowResponse, error) {
	// Create a Temporal client.
	temporalClient, err := client.NewClient(client.Options{})
	if err != nil {
		return nil, fmt.Errorf("unable to create Temporal client: %w", err)
	}
	defer temporalClient.Close()

	// Set workflow options. Here we generate a unique ID.
	workflowOptions := client.StartWorkflowOptions{
		ID:        "workflow-" + uuid.NewString(),
		TaskQueue: "data-processing-task-queue",
		// You can add timeouts or retry options as needed.
	}

	// Start the workflow asynchronously.
	we, err := temporalClient.ExecuteWorkflow(ctx, workflowOptions, DataProcessingWorkflow, req.InputData)
	if err != nil {
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}

	// Return the workflow and run IDs.
	return &workflowv1pb.StartWorkflowResponse{
		WorkflowId: we.GetID(),
		RunId:      we.GetRunID(),
	}, nil
}

// GetWorkflowStatus checks if the workflow has finished and returns the result or error.
func (s *workflowServiceServer) GetWorkflowStatus(ctx context.Context, req *workflowv1pb.GetWorkflowStatusRequest) (*workflowv1pb.GetWorkflowStatusResponse, error) {
	temporalClient, err := client.NewClient(client.Options{})
	if err != nil {
		return nil, fmt.Errorf("unable to create Temporal client: %w", err)
	}
	defer temporalClient.Close()

	// Obtain a handle to the workflow execution using its ID.
	workflowHandle := temporalClient.GetWorkflow(ctx, req.WorkflowId, "")

	var result string
	// Get blocks until the workflow completes.
	err = workflowHandle.Get(ctx, &result)
	if err != nil {
		// If Get() returns an error, for example because the workflow is still running,
		// you can return a status of RUNNING. (In production, you might distinguish between "still running" and a real failure.)
		return &workflowv1pb.GetWorkflowStatusResponse{
			Status: workflowv1pb.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
		}, nil
	}

	// If no error, the workflow is completed successfully.
	return &workflowv1pb.GetWorkflowStatusResponse{
		Status: workflowv1pb.WorkflowStatus_WORKFLOW_STATUS_COMPLETED,
		Result: result,
	}, nil
}

// DataProcessingWorkflow is a placeholder for your Temporal workflow definition.
// This would normally be defined in its own package using Temporal's SDK.
func DataProcessingWorkflow(ctx workflow.Context, input string) (string, error) {
	// Example logic: call several activities in sequence.
	// For brevity, we simply return a processed string.
	return "processed: " + input, nil
}

func main() {
	// Here you would wire up your ConnectRPC server with the workflowServiceServer.
	// Additionally, you need to start a Temporal worker to run your workflows.
	// This example shows a simple worker setup.

	// Create a Temporal client.
	temporalClient, err := client.NewClient(client.Options{})
	if err != nil {
		panic(fmt.Sprintf("unable to create Temporal client: %v", err))
	}
	defer temporalClient.Close()

	// Set up a worker to listen on the task queue.
	w := worker.New(temporalClient, "data-processing-task-queue", worker.Options{})

	// Register your workflow and any activities.
	w.RegisterWorkflow(DataProcessingWorkflow)
	// If you had activities, you would register them here as well.
	// w.RegisterActivity(YourActivityImplementation)

	// Start the worker in a separate goroutine.
	go func() {
		err = w.Run(worker.InterruptCh())
		if err != nil {
			panic(fmt.Sprintf("unable to start worker: %v", err))
		}
	}()

	// Set up your ConnectRPC server using the generated code.
	// This is pseudocode; your ConnectRPC server setup may vary.
	/*
		srv := connectrpc.NewServer()
		srv.Handle(new(workflowServiceServer))
		if err := srv.ListenAndServe(":8080"); err != nil {
		    panic(err)
		}
	*/
}
