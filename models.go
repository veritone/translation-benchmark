package main

import (
	"encoding/json"
	"time"

	"github.com/veritone/translation-benchmark/api"
	models "github.com/veritone/translation-benchmark/api"
)

// TDOChan the return type for each concurrency worker
type TDOChan struct {
	TDOID string
	err   error
	data  BenchmarkServiceResultArray
}

// BenchmarkServicePostPayload represent a payload to the local service
type BenchmarkServicePostPayload struct {
	EngineOutputs map[string]EngineOutput `json:"engineOutputs"`
	GroundTruth   string                  `json:"groundTruth"`
	TdoID         string                  `json:"tdoId,omitempty"`
	ScliteFQN     string                  `json:"scliteFQN,omitempty"`
}

// EngineOutput an engine output object to pass into the benchmark service
// TODO edit here
type EngineOutput struct {
	Output          string `json:"output"`
	EngineName      string `json:"engineName"`
	ModelID         string `json:"modelId"`
	AssetID         string `json:"assetId"`
	DeployedVersion int64  `json:"deployedVersion"`
}

// BenchmarkServiceResultArray the result of the benchmark service
type BenchmarkServiceResultArray []BenchmarkServiceResult

// BenchmarkServiceResult represent what the local service will return
type BenchmarkServiceResult struct {
	EngineID        string  `json:"engineId"`
	EngineName      string  `json:"engineName"`
	AssetID         string  `json:"assetId"`
	ModelID         string  `json:"modelId"`
	Accuracy        float64 `json:"accuracy"`
	Precision       float64 `json:"precision"`
	Recall          float64 `json:"recall"`
	WER             float64 `json:"wer"`
	DeployedVersion int64   `json:"deployedVersion"`
}

// BenchmarkEnginePayload the payload for this engine
type BenchmarkEnginePayload struct {
	Mode               string      `json:"mode,omitempty"`
	JobID              string      `json:"jobId,omitempty"`
	TaskID             string      `json:"taskId,omitempty"`
	TDOID              string      `json:"recordingId,omitempty"`
	OrganizationID     string      `json:"organizationId,omitempty"`
	Token              string      `json:"token,omitempty"`
	VeritoneAPIBaseURL string      `json:"veritoneApiBaseUrl,omitempty"`
	TaskPayload        TaskPayload `json:"taskPayload,omitempty"`
	Debug              bool        `json:"debug,omitempty"`
	Test               bool        `json:"test,omitempty"`
	HeartbeatWebhook   string      `json:"heartbeatWebhook,omitempty"`
	TheRest            json.RawMessage
}

// TaskPayload task payload
type TaskPayload struct {
	OrganizationID              int    `json:"organizationId"`
	Mode                        string `json:"mode"`
	TrainingWorkflowSDOID       string `json:"sdoId"`
	TrainingWorkflowSDOSchemaID string `json:"schemaId"`
	// ASSET BENCHMARKS
	BaselineAssetIDs []string `json:"baselineAssetIds"`
	AssetIDs         []string `json:"assetIds"`
	DataRegistryID   string   `json:"dataRegistryId"`
	CategoryID       string   `json:"categoryId"`
	MinPrecision     float64  `json:"minPrecision"`
}

// PayloadEngines what an array of PayloadEngine would be
type PayloadEngines []PayloadEngine

// PayloadEngine engine object from payload array
type PayloadEngine struct {
	EngineID string `json:"engineId"`
	ModelID  string `json:"modelId,omitempty"`
}

// TDOAssets organize the various assets by TDOID
type TDOAssets struct {
	assets        []*api.Asset
	baselineAsset *api.Asset
}

// BenchmarkSDOData a benchmark SDO object
type BenchmarkSDOData struct {
	Name                string       `json:"name"`
	TaskID              string       `json:"taskId,omitempty"`
	TDOID               string       `json:"tdoId"`
	Engines             string       `json:"engines"`
	Timestamp           int64        `json:"timestamp"`
	OrganizationID      string       `json:"organizationId,omitempty"`
	GroundTruthEngineID string       `json:"gtEngineId,omitempty"`
	FailedTDOs          []string     `json:"failedTDOs,omitempty"`
	SuccessTDOs         []string     `json:"successTDOs,omitempty"`
	IsAvg               bool         `json:"isAvg,omitempty"`
	TrainingJob         SDOReference `json:"trainingJob,omitempty"`
}

// AssetBenchmarkSDOData the asset benchmark SDO object
type AssetBenchmarkSDODataForTranscription struct {
	BenchmarkJobID   string `json:"benchmarkJobId"`
	BenchmarkTaskID  string `json:"benchmarkTaskId"`
	TDOID            string `json:"tdoId"`
	AssetID          string `json:"assetId"`
	ModelID          string `json:"modelId"`
	EngineID         string `json:"engineId"`
	EngineName       string `json:"engineName"`
	DeployedVersion  int64  `json:"deployedVersion"`
	OrganizationID   string `json:"organizationId"`
	BaselineAssetID  string `json:"baselineAssetId"`
	ProcessingTimeMS int64  `json:"processingTimeMs"`
	// Metrics
	Accuracy      float64 `json:"accuracy"`
	Precision     float64 `json:"precision"`
	Recall        float64 `json:"recall"`
	WordErrorRate float64 `json:"wordErrorRate"`
	// For SRC Training Workflow
	TrainingSDO *SDOReference `json:"trainingSdo,omitempty"`
	Words       []word        `json:"word"`
}

// AssetBenchmarkSDODataForFaceDetection the asset benchmark SDO object
type AssetBenchmarkSDODataForFaceDetection struct {
	BenchmarkJobID  string `json:"benchmarkJobId"`
	BenchmarkTaskID string `json:"benchmarkTaskId"`
	TDOID           string `json:"tdoId"`
	AssetID         string `json:"assetId"`
	CategoryID      string `json:"categoryId"`
	ModelID         string `json:"modelId"`
	EngineID        string `json:"engineId"`
	EngineName      string `json:"engineName"`
	DeployedVersion int64  `json:"deployedVersion"`
	OrganizationID  string `json:"organizationId"`
	BaselineAssetID string `json:"baselineAssetId"`
	// Metrics
	PrecisionCfg     float64         `json:"precisionCfg"`
	BaselineSeries   []models.Series `json:"baselineSeries"`
	Series           []models.Series `json:"series"`
	ProcessingTimeMs float64         `json:"processingTimeMs"`

	// For SRC Training Workflow
	TrainingSDO *SDOReference `json:"trainingSdo,omitempty"`
}

// SDOReference the reference to an SDO
type SDOReference struct {
	ID       string `json:"id"`
	SchemaID string `json:"schemaId"`
}

// VTNEngineOutput Engine output standard format for json.
type VTNEngineOutput struct {
	SchemaID            string                 `json:"schemaId,omitempty"`
	SourceEngineID      string                 `json:"sourceEngineId,omitempty"`
	SourceEngineName    string                 `json:"sourceEngineName,omitempty"`
	TaskPayload         map[string]interface{} `json:"taskPayload,omitempty"`
	TaskID              string                 `json:"taskId,omitempty"`
	GeneratedDateUTC    time.Time              `json:"generatedDateUTC,omitempty"`
	ExternalSourceID    string                 `json:"externalSourceId,omitempty"`
	ValidationContracts []string               `json:"validationContracts,omitempty"`
	Tags                []Tags                 `json:"tags,omitempty"`
	Language            string                 `json:"language,omitempty"`
	EntityID            string                 `json:"entityId,omitempty"`
	LibraryID           string                 `json:"libraryId,omitempty"`
	Series              []Series               `json:"series,omitempty"`
	Vendor              map[string]interface{} `json:"vendor,omitempty"`
	Partial             bool                   `json:"partial,omitempty"`
}

// Tags tags that appear in vtn output
type Tags struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// Series Data returned by engines
type Series struct {
	StartTimeMs int32  `json:"startTimeMs,omitempty"`
	StopTimeMs  int32  `json:"stopTimeMs,omitempty"`
	Tags        []Tags `json:"tags,omitempty"`
	Language    string `json:"language,omitempty"`
	SpeakerID   string `json:"speakerId,omitempty"`
	Words       []Word `json:"words,omitempty"`
	EntityID    string `json:"entityId,omitempty"`
	LibraryID   string `json:"libraryId,omitempty"`
}

// Word a word object
type Word struct {
	Word            string  `json:"word,omitempty"`
	Confidence      float64 `json:"confidence,omitempty"`
	BestPath        bool    `json:"bestPath,omitempty"`
	UtteranceLength int     `json:"utteranceLength,omitempty"`
}
