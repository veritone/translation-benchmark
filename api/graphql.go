package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/veritone/graphql"
)

// Setup stuff
const (
	defaultGraphQLEndpoint = "/v3/graphql"
	defaultTimeout         = "10s"

	tokenKey            contextKey = "token"
	correllationIDField            = "Veritone-Correlation-ID"
)

// Options some options for graphql
type Options struct {
	VeritoneAPIBaseURL string `json:"veritoneApiBaseUrl"`
	GraphQLEndpoint    string `json:"graphQLEndpoint"`
	Token              string `json:"token"`
	ClusterID          string `json:"clusterId"`
	TimeoutDurationStr string `json:"timeoutDurationStr"`
	Debug              bool   `json:"debug"`
	InstanceID         string `json:"instanceId"`
}

type contextKey string

func beforeRetryHandler(req *http.Request, resp *http.Response, err error, num int) {
	if err != nil {
		log.Printf("Retrying (attempt %d) after err: %s -- response: %+v", num, err, resp)
	} else {
		log.Printf("Retrying (attempt %d) after status: %s -- response: %+v", num, resp.Status, resp)
	}
}

func ToString(c interface{}) string {
	if c == nil {
		return ""
	}
	s, _ := json.MarshalIndent(c, "", "\t")
	return string(s)
}

func ToPlainString(c interface{}) string {
	if c == nil {
		return ""
	}
	s, _ := json.Marshal(c)
	return string(s)
}

// PlatformGraphQLClient the client
type PlatformGraphQLClient struct {
	*graphql.Client
}

func (options *Options) defaults() {
	if len(options.GraphQLEndpoint) == 0 {
		options.GraphQLEndpoint = defaultGraphQLEndpoint
	}

	if len(options.TimeoutDurationStr) == 0 {
		options.TimeoutDurationStr = defaultTimeout
	}
}

// NewCoreAPI new api
func NewCoreAPI(config Options) (*PlatformGraphQLClient, error) {
	if config.VeritoneAPIBaseURL == "" || config.Token == "" {
		return nil, fmt.Errorf("Missing connection info")
	}
	config.defaults()
	timeoutDur, err := time.ParseDuration(config.TimeoutDurationStr)
	if err != nil {
		return nil, fmt.Errorf(`invalid timeout given "%s": %s`, config.TimeoutDurationStr, err)
	}

	endpoint := config.VeritoneAPIBaseURL + config.GraphQLEndpoint
	cl := graphql.NewClient(endpoint,
		graphql.UseMultipartForm(),
		graphql.WithDefaultHeaders(getDefaultHeaders(config.InstanceID)),
		graphql.WithDefaultExponentialRetryConfig(),
		withAuthHeader(config.Token, timeoutDur),
		graphql.WithBeforeRetryHandler(beforeRetryHandler))

	if config.Debug {
		cl.Log = func(s string) { log.Println(s) }
	}

	return &PlatformGraphQLClient{Client: cl}, nil
}

func getDefaultHeaders(alternativeHost string) map[string]string {
	defaultHeaders := make(map[string]string)
	hostName, err := os.Hostname()
	if err != nil {
		log.Printf("Error getting host name: %s", err)
		hostName = alternativeHost
	}
	log.Printf("hostName: %s", hostName)

	defaultHeaders[correllationIDField] = hostName
	return defaultHeaders
}

func withAuthHeader(token string, timeout time.Duration) graphql.ClientOption {
	tr := &authHTTPTransport{
		Transport: &http.Transport{},
		token:     token,
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	return graphql.WithHTTPClient(client)
}

type authHTTPTransport struct {
	*http.Transport
	token string
}

func (t *authHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token := t.token

	if ctxToken := req.Context().Value(tokenKey); ctxToken != nil {
		if tk, ok := ctxToken.(string); ok && tk != "" {
			token = tk
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	return t.Transport.RoundTrip(req)
}

// FetchTDOOutputs fetch the assets of the given asset type from the given tdo
func (c *PlatformGraphQLClient) FetchTDOOutputs(ctx context.Context, tdoID string, assetType string) (*TDO, error) {
	if assetType == "" {
		assetType = "vtn-standard"
	}
	req := graphql.NewRequest(`
		query (
			$tdoId: ID!
			$assetType: [String!]
		) {
			temporalDataObject(id: $tdoId) {
				assets(type: $assetType, orderBy: createdDateTime, orderDirection: desc, limit: 100) {
					records {
						id
						sourceData {
							name
							taskId
							engineId
						}
						transform(transformFunction: JSON)
					}
				}
			}
		}
	`)

	req.Var("tdoId", tdoID)
	assetTypeArray := make([]string, 0)
	assetTypeArray = append(assetTypeArray, assetType)
	req.Var("assetType", assetTypeArray)

	var resp struct {
		Result *TDO `json:"temporalDataObject"`
	}

	return resp.Result, c.Run(ctx, req, &resp)
}

// FetchEngineResults fetch the engine output
func (c *PlatformGraphQLClient) FetchEngineResults(ctx context.Context, tdoID string, engineIDs []string) (*EngineResults, error) {
	req := graphql.NewRequest(`
		query (
			$tdoId: ID!
			$engineIds: [ID!]
		) {
			engineResults(tdoId: $tdoId, engineIds: $engineIds) {
				records {
					assetId
					tdoId
					engineId
					jsondata
				}
			}
		}
	`)

	req.Var("tdoId", tdoID)
	req.Var("engineIds", engineIDs)

	var resp struct {
		Result *EngineResults `json:"engineResults"`
	}

	return resp.Result, c.Run(ctx, req, &resp)
}

// FetchAsset fetch an asset
func (c *PlatformGraphQLClient) FetchAsset(ctx context.Context, assetID string) (*Asset, error) {
	req := graphql.NewRequest(`
		query (
			$assetId: ID!
		) {
			asset(id: $assetId) {
				id
				container {
					id
				}
				sourceData {
					taskId
					engine {
						name
						id
						deployedVersion
						categoryId
					}
				}
				transform(transformFunction: JSON)
			}
		}
	`)

	req.Var("assetId", assetID)

	var resp struct {
		Result *Asset `json:"asset"`
	}

	return resp.Result, c.Run(ctx, req, &resp)
}

// FetchEngine fetch an engine
func (c *PlatformGraphQLClient) FetchEngine(ctx context.Context, engineID string) (*Engine, error) {
	req := graphql.NewRequest(`
		query (
			$engineId: ID!
		) {
			engine(id: $engineId) {
				id
				name
				category {
					id
					name
				}
				deployedVersion
			}
		}
	`)

	req.Var("engineId", engineID)

	var resp struct {
		Result *Engine `json:"engine"`
	}
	return resp.Result, c.Run(ctx, req, &resp)
}

// FetchSchemas get the list of schemas given a data registry ID
func (c *PlatformGraphQLClient) FetchSchemas(ctx context.Context, dataRegistryID string) (*Schemas, error) {
	req := graphql.NewRequest(`
		query (
			$dataRegistryId: ID!
		) {
			dataRegistry(id: $dataRegistryId) {
				id
				schemas {
					records {
						id
						status
					}
				}
			}
		}
	`)

	req.Var("dataRegistryId", dataRegistryID)

	var resp struct {
		Result *Schemas `json:"dataRegistry"`
	}

	return resp.Result, c.Run(ctx, req, &resp)
}

// FetchPublishedSchema get the published schema given a data registry ID
func (c *PlatformGraphQLClient) FetchPublishedSchema(ctx context.Context, dataRegistryID string) (*PublishedSchema, error) {
	req := graphql.NewRequest(`
		query (
			$dataRegistryId: ID!
		) {
			dataRegistry(id: $dataRegistryId) {
				id
				publishedSchema {
					id
					status
					structuredDataObjects {
						records {
							id
							schemaId
							createdDateTime
							modifiedDateTime
							data
							dataString
						}
					}
				}
			}
		}
	`)

	req.Var("dataRegistryId", dataRegistryID)

	var resp struct {
		Result *PublishedSchema `json:"dataRegistry"`
	}

	return resp.Result, c.Run(ctx, req, &resp)
}

// CreateSDO create a structured data object in our platform
func (c *PlatformGraphQLClient) CreateSDO(ctx context.Context, schemaID string, data interface{}) (*SDO, error) {
	req := graphql.NewRequest(`
		mutation (
			$schemaId: ID!
			$data: JSONData
		) {
			createStructuredData(input: {
				schemaId: $schemaId
				data: $data
			}) {
				id
				schemaId
				createdDateTime
				modifiedDateTime
				data
				dataString
			}
		}
	`)
	req.Var("schemaId", schemaID)
	req.Var("data", data)

	var resp struct {
		Result *SDO `json:"createStructuredData"`
	}

	return resp.Result, c.Run(ctx, req, &resp)
}

// CreateJob create a job in our platform
func (c *PlatformGraphQLClient) CreateJob(ctx context.Context, tdoID string, isReprocessJob bool, tasks ...CreateJobTask) (*Job, error) {
	req := graphql.NewRequest(`
		mutation(
			$targetId: ID
			$isReprocessJob: Boolean
			$tasks: [CreateTask!]
		) {
			createJob(input: {
				targetId: $targetId
				isReprocessJob: $isReprocessJob
				tasks: $tasks
			}) {
				id
				name
				targetId
				status
				tasks {
					records {
						id
						status
						engine {
							id
							name
						}
					}
				}
			}
		}
	`)

	req.Var("targetId", tdoID)
	req.Var("isReprocessJob", isReprocessJob)
	req.Var("tasks", tasks)

	var resp struct {
		Result *Job `json:"createJob"`
	}
	return resp.Result, c.Run(ctx, req, &resp)
}

// FetchJob get the job
func (c *PlatformGraphQLClient) FetchJob(ctx context.Context, jobID string) (*Job, error) {
	req := graphql.NewRequest(`
		query(
			$jobId: ID!
		) {
			job(id: $jobId) {
				id
				name
				status
				targetId
				clusterId
				tasks {
					records {
						id
						status
						engine {
							id
							name
						}
					}
				}
			}
		}`)

	req.Var("jobId", jobID)

	var resp struct {
		Result *Job `json:"job"`
	}

	return resp.Result, c.Run(ctx, req, &resp)
}

// AppendWarningToTask append a warning to the running task output
func (c *PlatformGraphQLClient) AppendWarningToTask(ctx context.Context, taskID string, assetID string, reason string, message string) error {
	req := graphql.NewRequest(`
	mutation(
		$taskId: ID
		$referenceId: ID
		$reason: String!
		$message: String
	) {
		appendWarningToTask(
			taskId: $taskId
			reason: $reason
			message: $message
			referenceId: $referenceId
		)
	}`)

	req.Var("taskId", taskID)
	req.Var("referenceId", assetID)
	req.Var("reason", reason)
	req.Var("message", message)

	var resp struct {
		Result interface{}
	}

	return c.Run(ctx, req, &resp)
}
