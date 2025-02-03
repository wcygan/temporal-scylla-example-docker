package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	// Replace with your generated package import path.
	"buf.build/gen/go/wcygan/temporal-scylla-example/connectrpc/go/workflow/v1/workflowv1connect"
	workflowv1pb "buf.build/gen/go/wcygan/temporal-scylla-example/protocolbuffers/go/workflow/v1"
)

// workflowServiceServer implements the WorkflowService defined in our proto.
type workflowServiceServer struct {
	workflowv1connect.UnimplementedWorkflowServiceHandler
}

// StartWorkflow starts a new Temporal workflow and returns its identifiers.
func (s *workflowServiceServer) StartWorkflow(ctx context.Context, req *connect.Request[workflowv1pb.StartWorkflowRequest]) (*connect.Response[workflowv1pb.StartWorkflowResponse], error) {
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
	we, err := temporalClient.ExecuteWorkflow(ctx, workflowOptions, DataProcessingWorkflow, req.Msg.InputData)
	if err != nil {
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}

	// Return the workflow and run IDs.
	return connect.NewResponse(&workflowv1pb.StartWorkflowResponse{
		WorkflowId: we.GetID(),
		RunId:      we.GetRunID(),
	}), nil
}

// GetWorkflowStatus checks if the workflow has finished and returns the result or error.
func (s *workflowServiceServer) GetWorkflowStatus(ctx context.Context, req *connect.Request[workflowv1pb.GetWorkflowStatusRequest]) (*connect.Response[workflowv1pb.GetWorkflowStatusResponse], error) {
	temporalClient, err := client.NewClient(client.Options{})
	if err != nil {
		return nil, fmt.Errorf("unable to create Temporal client: %w", err)
	}
	defer temporalClient.Close()

	// Obtain a handle to the workflow execution using its ID.
	workflowHandle := temporalClient.GetWorkflow(ctx, req.Msg.WorkflowId, "")

	var result string
	// Get blocks until the workflow completes.
	err = workflowHandle.Get(ctx, &result)
	if err != nil {
		// If Get() returns an error, for example because the workflow is still running,
		// you can return a status of RUNNING. (In production, you might distinguish between "still running" and a real failure.)
		return connect.NewResponse(&workflowv1pb.GetWorkflowStatusResponse{
			Status: workflowv1pb.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
		}), nil
	}

	// If no error, the workflow is completed successfully.
	return connect.NewResponse(&workflowv1pb.GetWorkflowStatusResponse{
		Status: workflowv1pb.WorkflowStatus_WORKFLOW_STATUS_COMPLETED,
		Result: result,
	}), nil
}

// DataProcessingWorkflow is a placeholder for your Temporal workflow definition.
// This would normally be defined in its own package using Temporal's SDK.
func DataProcessingWorkflow(ctx workflow.Context, input string) (string, error) {
	// Example logic: call several activities in sequence.
	// For brevity, we simply return a processed string.
	return "processed: " + input, nil
}

// loggingInterceptor logs incoming requests
func loggingInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			duration := time.Since(start)

			// Log the request details
			fmt.Printf("Request: %s, Duration: %v, Error: %v\n",
				req.Spec().Procedure,
				duration,
				err,
			)

			return resp, err
		}
	}
}

func main() {
	// Get port from environment or use default
	port := "8081"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	// Create a Temporal client
	temporalClient, err := client.NewClient(client.Options{})
	if err != nil {
		panic(fmt.Sprintf("unable to create Temporal client: %v", err))
	}
	defer temporalClient.Close()

	// Set up a worker to listen on the task queue
	w := worker.New(temporalClient, "data-processing-task-queue", worker.Options{})

	// Register workflow and activities
	w.RegisterWorkflow(DataProcessingWorkflow)

	// Start the worker in a separate goroutine
	go func() {
		err = w.Run(worker.InterruptCh())
		if err != nil {
			panic(fmt.Sprintf("unable to start worker: %v", err))
		}
	}()

	// Create a new HTTP mux for routing
	mux := http.NewServeMux()

	// Common interceptors for all handlers
	interceptors := connect.WithInterceptors(loggingInterceptor())

	// Register the workflow service
	workflowPath, workflowHandler := workflowv1connect.NewWorkflowServiceHandler(
		&workflowServiceServer{},
		interceptors,
	)
	mux.Handle(workflowPath, workflowHandler)

	// Register health check service
	healthPath, healthHandler := grpchealth.NewHandler(grpchealth.NewStaticChecker(), interceptors)
	mux.Handle(healthPath, healthHandler)

	// Register reflection service
	reflector := grpcreflect.NewStaticReflector(
		workflowv1connect.WorkflowServiceName, // Your service name
		grpchealth.HealthV1ServiceName,        // Health service name
	)
	reflectPath, reflectHandler := grpcreflect.NewHandlerV1(reflector)
	mux.Handle(reflectPath, reflectHandler)

	// Add a simple HTTP health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap the mux with h2c for HTTP/2 support
	h2cHandler := h2c.NewHandler(mux, &http2.Server{})

	// Create the server
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           h2cHandler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Start the server
	fmt.Printf("Starting server on :%s\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("Failed to start server: %v", err))
	}
}
