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

// Global Temporal client instance.
var temporalClient client.Client

// newTemporalClient reads environment variables and returns a new Temporal client.
func newTemporalClient() (client.Client, error) {
	temporalHost := os.Getenv("TEMPORAL_HOST")
	if temporalHost == "" {
		temporalHost = "localhost"
	}
	temporalPort := os.Getenv("TEMPORAL_PORT")
	if temporalPort == "" {
		temporalPort = "7233"
	}
	temporalAddress := fmt.Sprintf("%s:%s", temporalHost, temporalPort)
	return client.NewClient(client.Options{
		HostPort: temporalAddress,
	})
}

// workflowServiceServer implements the WorkflowService defined in our proto.
type workflowServiceServer struct {
	workflowv1connect.UnimplementedWorkflowServiceHandler
}

// StartWorkflow starts a new Temporal workflow and returns its identifiers.
func (s *workflowServiceServer) StartWorkflow(ctx context.Context, req *connect.Request[workflowv1pb.StartWorkflowRequest]) (*connect.Response[workflowv1pb.StartWorkflowResponse], error) {
	// Set workflow options. Here we generate a unique ID.
	workflowOptions := client.StartWorkflowOptions{
		ID:        "workflow-" + uuid.NewString(),
		TaskQueue: "data-processing-task-queue",
	}
	we, err := temporalClient.ExecuteWorkflow(ctx, workflowOptions, DataProcessingWorkflow, req.Msg.InputData)
	if err != nil {
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}
	return connect.NewResponse(&workflowv1pb.StartWorkflowResponse{
		WorkflowId: we.GetID(),
		RunId:      we.GetRunID(),
	}), nil
}

// GetWorkflowStatus checks the workflow's completion status and returns the result.
func (s *workflowServiceServer) GetWorkflowStatus(ctx context.Context, req *connect.Request[workflowv1pb.GetWorkflowStatusRequest]) (*connect.Response[workflowv1pb.GetWorkflowStatusResponse], error) {
	workflowHandle := temporalClient.GetWorkflow(ctx, req.Msg.WorkflowId, "")

	var result string
	err := workflowHandle.Get(ctx, &result)
	if err != nil {
		// The workflow might still be running.
		return connect.NewResponse(&workflowv1pb.GetWorkflowStatusResponse{
			Status: workflowv1pb.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
		}), nil
	}

	return connect.NewResponse(&workflowv1pb.GetWorkflowStatusResponse{
		Status: workflowv1pb.WorkflowStatus_WORKFLOW_STATUS_COMPLETED,
		Result: result,
	}), nil
}

// DataProcessingWorkflow is a placeholder for your Temporal workflow.
func DataProcessingWorkflow(ctx workflow.Context, input string) (string, error) {
	return "processed: " + input, nil
}

// loggingInterceptor logs incoming requests.
func loggingInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			fmt.Printf("Request: %s, Duration: %v, Error: %v\n",
				req.Spec().Procedure,
				time.Since(start),
				err)
			return resp, err
		}
	}
}

func connectToTemporal(maxRetries int, delay time.Duration) client.Client {
	var temporalClient client.Client
	var err error

	for i := 0; i < maxRetries; i++ {
		temporalClient, err = newTemporalClient()
		if err == nil {
			fmt.Println("Successfully connected to Temporal server")
			return temporalClient
		}
		fmt.Printf("Attempt %d: Failed to connect to Temporal server: %v\n", i+1, err)
		time.Sleep(delay)
	}

	panic(fmt.Sprintf("unable to create Temporal client after %d attempts: %v", maxRetries, err))
}

func main() {
	// Get listen port from environment or use default 8081.
	port := "8081"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	// Retry connecting to Temporal server (20 attempts, 3 seconds apart)
	temporalClient = connectToTemporal(20, 3*time.Second)
	defer temporalClient.Close()

	// Set up Temporal worker.
	workerInstance := worker.New(temporalClient, "data-processing-task-queue", worker.Options{})
	workerInstance.RegisterWorkflow(DataProcessingWorkflow)
	go func() {
		err := workerInstance.Run(worker.InterruptCh())
		if err != nil {
			panic(fmt.Sprintf("unable to start worker: %v", err))
		}
	}()

	// Create HTTP multiplexer for ConnectRPC.
	mux := http.NewServeMux()
	interceptors := connect.WithInterceptors(loggingInterceptor())

	workflowPath, workflowHandler := workflowv1connect.NewWorkflowServiceHandler(
		&workflowServiceServer{},
		interceptors,
	)
	mux.Handle(workflowPath, workflowHandler)

	healthPath, healthHandler := grpchealth.NewHandler(grpchealth.NewStaticChecker(), interceptors)
	mux.Handle(healthPath, healthHandler)

	reflector := grpcreflect.NewStaticReflector(
		workflowv1connect.WorkflowServiceName,
		grpchealth.HealthV1ServiceName,
	)
	reflectPath, reflectHandler := grpcreflect.NewHandlerV1(reflector)
	mux.Handle(reflectPath, reflectHandler)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	h2cHandler := h2c.NewHandler(mux, &http2.Server{})

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           h2cHandler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	fmt.Printf("Starting server on :%s\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("Failed to start server: %v", err))
	}
}