package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/urfave/cli"

	"github.com/veritone/src-training-workflow/platform"
	"github.com/veritone/src-training-workflow/platform/persistence"
	"github.com/veritone/translation-benchmark/api"
)

const (
	// SigIntExitCode int exit code
	SigIntExitCode = 130
	// SigTermExitCode term exit code
	SigTermExitCode = 143
	// BenchmarkEngineID this engine ID
	benchmarkEngineID     = "2517dfe9-b70d-43b1-bc1b-800618190d92"
	categoryTranslationID = "3b2b2ff8-44aa-4db4-9b71-ff96c3bf5923"

	// Default MinPrecision: 40
	defaultMinPrecision = float64(40)
	serviceName         = "translation-benchmark"
)

type UpdateStatus struct {
	Status         string `json:"status,omitempty"`
	InfoMsg        string `json:"infoMsg,omitempty"`
	FailureReason  string `json:"failureReason,omitempty"`
	FailureMessage string `json:"failureMsg,omitempty"`
}

type Response struct {
	EstimatedProcessingTimeInSeconds int `json:"estimatedProcessingTimeInSeconds,omitempty"`
}

func main() {
	fmt.Println("Starting engine server host...")
	if err := http.ListenAndServe("0.0.0.0:8080", newServer()); err != nil {
		fmt.Println("Failed to starting engine server host...")
		fmt.Println(fmt.Sprintf("Error: %v", err))
		fmt.Fprintf(os.Stderr, "%s", err)
	}
}

func newServer() *http.ServeMux {
	s := http.NewServeMux()
	s.HandleFunc("/readyz", handleReady)
	s.HandleFunc("/process", handleProcess)
	return s
}

func handleReady(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func handleProcess(w http.ResponseWriter, r *http.Request) {
	myAppContext.App = cli.NewApp()
	myAppContext.StartTime = time.Now()
	myAppContext.App.Name = serviceName
	myAppContext.App.Usage = "Benchmark Translation Engines"
	myAppContext.App.Version = "0.0.1 (" + runtime.Version() + ")"

	myAppContext.App.Action = func(c *cli.Context) {
		log.Println("Start process benchmark translation engine")
		var err error
		payload := r.FormValue("payload")
		var heartbeatWebhook = r.FormValue("heartbeatWebhook")
		fmt.Println("heartbeatWebhook: ", heartbeatWebhook)

		if payload == "" {
			updateTaskStatusV3F("failed", "", "The `payload` is undefined  or empty.", "invalid_data", heartbeatWebhook)
			return
		}
		fmt.Printf("Loading payload from %s\n", payload)

		myEnginePayload = BenchmarkEnginePayload{}
		if err := json.Unmarshal([]byte(payload), &myEnginePayload); err != nil {
			updateTaskStatusV3F("failed", "", "Unable to unmarshal payload: "+err.Error(), "invalid_data", heartbeatWebhook)
			return
		}

		myEnginePayload.HeartbeatWebhook = heartbeatWebhook
		maxTTL, err := strconv.Atoi(r.FormValue("maxTTL"))
		if err != nil {
			updateTaskStatusV3F("failed", "", "Failed to parse maxTTL value: "+err.Error(), "invalid_data", heartbeatWebhook)
			return
		}

		// Response for the end func
		resp := Response{
			EstimatedProcessingTimeInSeconds: maxTTL,
		}

		defer func() {
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				fmt.Fprintf(os.Stderr, "%s", err)
			}
			fmt.Printf("Engine Exit successfully.")
		}()
		// End: Response for the end func

		myConfig = loadEngineWrapperConfigFile()
		myConfig.APIOptions.Token = myEnginePayload.Token
		myConfig.APIOptions.VeritoneApiBaseUrl = myEnginePayload.VeritoneAPIBaseURL
		// TODO: confirm before remove local api
		myConfig.LocalAPIOptions.Token = myEnginePayload.Token
		myConfig.LocalAPIOptions.VeritoneAPIBaseURL = myEnginePayload.VeritoneAPIBaseURL

		// Default to use Translation
		if myEnginePayload.TaskPayload.DataRegistryID == "" {
			myEnginePayload.TaskPayload.DataRegistryID = myConfig.DataRegistryIDs.Translation
			myEnginePayload.TaskPayload.CategoryID = categoryTranslationID
		}

		// Check Category
		if myEnginePayload.TaskPayload.CategoryID == "" {
			myEnginePayload.TaskPayload.CategoryID = categoryTranslationID
		}

		// Check MinPrecision
		if myEnginePayload.TaskPayload.MinPrecision < 0 {
			myEnginePayload.TaskPayload.MinPrecision = defaultMinPrecision
		}

		// Add config to current context
		myAppContext.Config = myConfig

		// let's get the API
		myAppContext.GraphQLClient, err = platform.NewCoreApi(myConfig.APIOptions)
		if err != nil {
			updateTaskStatusV3F("failed", "", "(GraphQLClient) Failed to get connection to Veritone platform: "+err.Error(), "invalid_data", heartbeatWebhook)
			return
		}

		// Local api implementation vs. platform api from src-training-workflow...this is so you don't have to make changes in src-training-workflow for things used only here
		myAppContext.LocalGraphQLClient, err = api.NewCoreAPI(myConfig.LocalAPIOptions)
		if err != nil {
			updateTaskStatusV3F("failed", "", "(LocalGraphQLClient) Failed to get connection to Veritone platform: "+err.Error(), "invalid_data", heartbeatWebhook)
			return
		}

		// set up stuff for shutting down handling due to signal or errors
		gracefulShutdownCtx, gracefulShutdownCancelFn := context.WithCancel(context.Background())
		var jobProcessingWaitGroup sync.WaitGroup
		go listenForSignals(gracefulShutdownCancelFn, &jobProcessingWaitGroup)

		err = invokeService(gracefulShutdownCtx, myAppContext.LocalGraphQLClient, &myEnginePayload)
		if err != nil {

			fmt.Printf("[ERROR]: Failed to benchmark -- err=%s\n", err)
			// exitCode = 1

			// Update task status
			updateTaskStatusV3F("failed", "", "Failed to benchmark: "+err.Error(), "internal_error", heartbeatWebhook)

			return
		}

		// Update task status
		updateTaskStatusV3F("complete", "", "", "", heartbeatWebhook)

		return
	}
}

func updateTaskStatusV3F(taskStatus, infoMsg, failureMessage, failureReason, webhook string) {
	fmt.Println(fmt.Sprintf("(updateTaskStatusV3F) updating task status to %s:", taskStatus))
	if err := updateTaskStatus(taskStatus, infoMsg, failureMessage, failureReason, webhook); err != nil {
		fmt.Println("(updateTaskStatusV3F) error:", err)
	}
}

// listenForSignals waits for SIGINT or SIGTERM to be captured.
// When caught, it shuts down gracefully and exits with the proper code.
func listenForSignals(cancelFunc context.CancelFunc, jobProcessingWaitGroup *sync.WaitGroup) {
	// Block until signal is caught
	notifyChan := make(chan os.Signal, 2)
	signal.Notify(notifyChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-notifyChan
	exitCode := 1
	switch sig {
	case syscall.SIGINT:
		exitCode = SigIntExitCode
	case syscall.SIGTERM:
		exitCode = SigTermExitCode
	}
	// Emit shutdown event, shutdown gracefully, exit with proper code
	fmt.Println("Signal Shutdown")
	gracefulShutdown(exitCode, cancelFunc, jobProcessingWaitGroup)

}

// gracefulShutdown cleans up anything worth cleaning before exiting
func gracefulShutdown(exitCode int, cancelFunc context.CancelFunc, jobProcessingWaitGroup *sync.WaitGroup) {
	fmt.Printf("Shutting down gracefully\n")
	// This call triggers the job processing worker to finish off its remaining work.
	cancelFunc()
	// When the job processing worker finishes off its remaining work, it will notify this goroutine here
	// and allow it to continue.

	jobProcessingWaitGroup.Wait()

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
