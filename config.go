package main

import (
	"time"

	"github.com/urfave/cli"
	"github.com/veritone/src-training-workflow/platform"
	"github.com/veritone/src-training-workflow/platform/persistence"
	"github.com/veritone/translation-benchmark/api"
)

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
