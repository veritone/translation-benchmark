package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/urfave/cli"
	"github.com/veritone/src-training-workflow/platform"
	"github.com/veritone/translation-benchmark/api"
)

const (
	// SigIntExitCode int exit code
	SigIntExitCode = 130
	// SigTermExitCode term exit code
	SigTermExitCode = 143
	// BenchmarkEngineID this engine ID
	benchmarkEngineID         = "6181fd6e-c6e1-44e8-afd3-75b1a8babd08"
	categoryTranscriptionID   = "67cd4dd0-2f75-445d-a6f0-2f297d6cd182"
	categoryFacialDetectionID = "6faad6b7-0837-45f9-b161-2f6bf31b7a07"
	categoryTranslationID     = "3b2b2ff8-44aa-4db4-9b71-ff96c3bf5923"

	// Default MinPrecision: 40
	defaultMinPrecision = float64(40)
	serviceName         = "translation-benchmark"
)

var (
	myServer        *httptest.Server
	myConfig        = ManagerConfig{}
	myEnginePayload = BenchmarkEnginePayload{}
	myAppContext    = AppContext{}
)

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
		resp := &api.Response{
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
		gracefulShutdownCtx, _ := context.WithCancel(context.Background())
		// var jobProcessingWaitGroup sync.WaitGroup
		// go listenForSignals(gracefulShutdownCancelFn, &jobProcessingWaitGroup)

		err = invokeService(gracefulShutdownCtx, myAppContext.LocalGraphQLClient, &myEnginePayload)
		if err != nil {

			fmt.Printf("[ERROR]: Failed to benchmark -- err=%s\n", err)
			// exitCode = 1

			// Update task status
			updateTaskStatusV3F("failed", "", "Failed to benchmark: "+err.Error(), "internal_error", heartbeatWebhook)

			return
		}

		// Update task status
		updateTaskStatusV3F("complete", "Engine run successfully", "", "", heartbeatWebhook)

		return
	}
}

func updateTaskStatusV3F(taskStatus, infoMsg, failureMessage, failureReason, webhook string) error {
	updateStatus := &api.UpdateStatus{
		Status:         taskStatus,
		InfoMsg:        infoMsg,
		FailureReason:  failureReason,
		FailureMessage: failureMessage,
	}
	b, err := json.Marshal(updateStatus)
	if err != nil {
		fmt.Println("error:", err)
		return err
	}

	req, err := http.NewRequest("POST", webhook, bytes.NewBuffer(b))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Printf("Success when UpdateTask to %s with status: %s, InfoMsg: %s, FailureReason: %s, FailureMessage: %s", resp.Status, taskStatus, infoMsg, failureReason, failureMessage)
	return nil
}

func loadEngineWrapperConfigFile() ManagerConfig {
	res := ManagerConfig{
		LocalServiceURL:   "http://localhost:35000",
		LocalServiceCmd:   "python3 /app/main.py --port 35000",
		LocalServiceRetry: 5}
	configFile := os.Getenv("CONFIG_FILE")
	if configFile != "" {
		reader, err := os.Open(configFile)
		if err == nil {
			defer reader.Close()
			err = json.NewDecoder(reader).Decode(&res)
		}
	} else {
		reader, err := os.Open("./config.json")
		if err == nil {
			defer reader.Close()
			err = json.NewDecoder(reader).Decode(&res)
		}
	}
	// still need to read from command line
	if localServiceCmd := os.Getenv("LOCAL_SERVICE_CMD"); localServiceCmd != "" {
		res.LocalServiceCmd = localServiceCmd
	}
	if localServiceURL := os.Getenv("LOCAL_SERVICE_URL"); localServiceURL != "" {
		res.LocalServiceURL = localServiceURL
	}
	if benchmarkID := os.Getenv("ENGINE_ID"); benchmarkID != "" {
		res.EngineID = benchmarkID
	} else {
		res.EngineID = benchmarkEngineID
	}

	return res
}
