package main

import (
	"time"

	"github.com/urfave/cli"
	GraphqlAPI "github.com/veritone/graphql-client-go"
	"github.com/veritone/src-training-workflow/platform"
	"github.com/veritone/src-training-workflow/platform/persistence"
	"github.com/veritone/translation-benchmark/api"
)

type Config struct {
	PrometheusPort   int                      `json:"prometheusPort"`
	LogFormat        string                   `json:"logFormat"`
	LogLevel         string                   `json:"logLevel"`
	EngineID         string                   `json:"engineId"`
	EngineInstanceID string                   `json:"engineInstanceId"`
	MaxConcurrency   int                      `json:"maxConcurrency"`
	MaxWorkersCount  int                      `json:"maxWorkersCount"`
	VeritoneBaseUri  string                   `json:"veritoneBaseUri"`
	TTLinSec         time.Duration            `json:"ttl"`
	GraphQLConfig    GraphqlAPI.GraphqlConfig `json:"graphql-api"`
	TemporaryTxt     string                   `json:"tempTxt"`
}

// ManagerConfig config
type ManagerConfig struct {
	LocalServiceCmd   string              `json:"localService"`
	LocalServiceURL   string              `json:"localServiceUrl"`
	LocalServiceRetry int                 `json:"localServiceRetry"`
	EngineID          string              `json:"engineId"`
	APIOptions        platform.ApiOptions `json:"api"`
	LocalAPIOptions   api.Options         `json:"localApi"`
	DataRegistryIDs   DataRegistryIDs     `json:"dataRegistryIds"`
}

// AppContext the context
type AppContext struct {
	App *cli.App

	StartTime time.Time
	// these are actualy derived from runtime, not a static config
	BenchmarkServiceChan chan *BenchmarkServiceResultArray

	GraphQLClient       *platform.PlatformGraphQLClient
	LocalGraphQLClient  *api.PlatformGraphQLClient
	SDOAPIClient        persistence.SDOAPI
	TrainingWorkflowObj persistence.TrainingWorkflowSDO
	TrainingWorkflowRef persistence.SDOReference
	Config              ManagerConfig
}

// DataRegistryIDs ID for Transcription and FaceDetection
type DataRegistryIDs struct {
	Translation   string `json:"translation"`
	Transcription string `json:"transcription"`
	FaceDetection string `json:"faceDetection"`
}
