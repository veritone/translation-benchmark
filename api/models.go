package api

// TDOs multiple TDO objects
type TDOs []TDO

// TDO a temporal data object
type TDO struct {
	ID     string       `json:"id"`
	Name   string       `json:"name,omitempty"`
	Assets AssetRecords `json:"assets,omitempty"`
}

// AssetRecords records of assets
type AssetRecords struct {
	Records []Asset `json:"records,omitempty"`
}

// Asset an asset
type Asset struct {
	ID         string     `json:"id,omitempty"`
	Container  TDO        `json:"container,omitempty"`
	SourceData SourceData `json:"sourceData,omitempty"`
	SignedURI  string     `json:"signedUri,omitempty"`
	Raw        string     `json:"transform"`
	Data       *EngineOutput
	Transcript string
	ModelID    string
}

// SourceData the source data for an asset
type SourceData struct {
	TaskID string  `json:"taskId,omitempty"`
	Engine *Engine `json:"engine,omitempty"`
}

// EngineResults engine results
type EngineResults struct {
	Records []EngineResult `json:"records,omitempty"`
}

// EngineResult engine result
type EngineResult struct {
	AssetID    string       `json:"assetId"`
	EngineID   string       `json:"engineId"`
	TDOID      string       `json:"tdoId"`
	EngineName string       `json:"engineName"`
	TaskID     string       `json:"taskId"`
	Data       EngineOutput `json:"jsondata"`
}

// EngineOutput output from an engine
type EngineOutput struct {
	TaskID           string   `json:"taskId,omitempty"`
	GeneratedDateUTC string   `json:"generatedDateUTC,omitempty"`
	Tags             []Tags   `json:"tags,omitempty"`
	Series           []Series `json:"series,omitempty"`
}

// Tags tag
type Tags struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// Series Data returned by engines
type Series struct {
	// Common
	StartTimeMs int32 `json:"startTimeMs,omitempty"`
	StopTimeMs  int32 `json:"stopTimeMs,omitempty"`

	// For transcription engine
	Language  string `json:"language,omitempty"`
	SpeakerID string `json:"speakerId,omitempty"`
	Words     []Word `json:"words,omitempty"`
	EntityID  string `json:"entityId,omitempty"`
	LibraryID string `json:"libraryId,omitempty"`

	// For Detection engine
	Object          SeriObject  `json:"object,omitempty"`
	IsOverlap       bool        `json:"isOverlap,omitempty"`
	OverlapPercent  float64     `json:"overlapPercent,omitempty"`
	IsMatch         bool        `json:"isMatch,omitempty"`
	IsFalsePostive  bool        `json:"isFalsePostive,omitempty"`
	IsFalseNegative bool        `json:"isFalseNegative,omitempty"`
	BaseLineSerie   interface{} `json:"baseLineSerie,omitempty"`
}

type SeriObject struct {
	Type         string    `json:"type,omitempty"`
	Confidence   float64   `json:"confidence,omitempty"`
	URI          string    `json:"uri,omitempty"`
	PoundingPoly []Point   `json:"boundingPoly,omitempty"`
	Rectangle    Rectangle `json:"rectangle,omitempty"`
}

// Word a word in output
type Word struct {
	Word            string  `json:"word,omitempty"`
	Confidence      float64 `json:"confidence,omitempty"`
	BestPath        bool    `json:"bestPath,omitempty"`
	UtteranceLength int     `json:"utteranceLength,omitempty"`
}

// Engine an engine
type Engine struct {
	ID              string    `json:"id"`
	Name            string    `json:"name,omitempty"`
	Category        *Category `json:"category,omitempty"`
	Description     string    `json:"description,omitempty"`
	DeployedVersion int64     `json:"deployedVersion,omitempty"`
	CategoryID      string    `json:"categoryId,omitempty"`
}

// Category an engine category
type Category struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Schema represent a schema
type Schema struct {
	ID         string      `json:"id"`
	Status     string      `json:"status"`
	SDORecords *SDORecords `json:"structuredDataObjects,omitempty"`
}

// SDORecords represent a list of SDO records
type SDORecords struct {
	SDOs []SDO `json:"records"`
}

// SchemaRecords schema records from graphql
type SchemaRecords struct {
	Records []Schema `json:"records"`
}

// Schemas represent the schemas response
type Schemas struct {
	Schemas SchemaRecords `json:"schemas"`
}

// PublishedSchema represent one published schema
type PublishedSchema struct {
	DataRegistryID string  `json:"id"`
	Schema         *Schema `json:"publishedSchema,omitempty"`
}

// SDO represent the SDO response from graphql
type SDO struct {
	ID               string                 `json:"id"`
	SchemaID         string                 `json:"schemaId"`
	CreatedDataTime  string                 `json:"createdDateTime,omitempty"`
	ModifiedDateTime string                 `json:"modifiedDateTime,omitempty"`
	Data             map[string]interface{} `json:"data,omitempty"`
	DataString       string                 `json:"dataString,omitempty"`
}

// Job represent a job
type Job struct {
	// TdoID     string `json:"targetId,omitempty"`
	JobID     string `json:"id"`
	TDOID     string `json:"targetId,omitempty"`
	Name      string `json:"name,omitempty"`
	Status    string `json:"status,omitempty"`
	ClusterID string `json:"clusterId,omitempty"`
	Tasks     Tasks  `json:"tasks,omitempty"`
}

// JobRecords represent the record of jobs from the `jobs` query
type JobRecords struct {
	Jobs []Job `json:"records"`
}

// CreateJobTask represent a task when creating a job
type CreateJobTask struct {
	EngineID string                 `json:"engineId"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
}

// Tasks a list of tasks
type Tasks struct {
	Records []Task `json:"records,omitempty"`
}

// Task represent a task
type Task struct {
	TaskID string `json:"id"`
	Status string `json:"status,omitempty"`
	Engine Engine `json:"engine"`
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Rectangle struct {
	// Top left
	TL Point `json:"tl,omitempty"`
	// Bottom right
	BR Point `json:"br,omitempty"`
	// Superficies
	S      float64 `json:"s,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}
