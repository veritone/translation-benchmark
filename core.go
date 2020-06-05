package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	uuid "github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/veritone/translation-benchmark/api"
)

const (
	cieloEngineID = "8ab12d01-6ad5-444f-8fc3-a73489aa6e08"
	srcEngineID   = "9203cf30-68d1-4978-a5a4-7f2f3a945ccb"
)

var (
	pollCount        = 360          // means polling every 40 seconds based on a 4 hr timeout
	pollTimeoutInSec = int64(14400) // default timeout to 4 hours
)

type results struct {
	Correct     int `json:"correct"`
	Substituted int `json:"substituted"`
	Deleted     int `json:"deleted"`
	Inserted    int `json:"inserted"`
	WordCount   int `json:"wordCount"`

	Accuracy      float64 `json:"accuracy"`
	Recall        float64 `json:"recall"`
	Precision     float64 `json:"precision"`
	WordErrorRate float64 `json:"wordErrorRate"`

	Words []word `json:"words,omitempty"`

	EngineID        string `json:"engineId"`
	EngineName      string `json:"engineName"`
	AssetID         string `json:"assetId"`
	ModelID         string `json:"modelId"`
	DeployedVersion int64  `json:"deployedVersion"`
}
type word struct {
	Action     string `json:"action,omitempty"`
	Reference  string `json:"reference,omitempty"`
	Hypothesis string `json:"hypothesis,omitempty"`
}

// invokeService is the core logic entrypoint for the engine. It will setup the payload data accordingly,
// pass it to the benchmark engine, and generate the benchmark SDO
func invokeService(shutdownCtx context.Context, graphQLClient *api.PlatformGraphQLClient, enginePayload *BenchmarkEnginePayload) (err error) {
	var benchmarkDataRegistryID = enginePayload.TaskPayload.DataRegistryID
	var benchmarkSchemaID string

	// Check that the payload has assets in it. There must be at least 1 asset in both payload fields.
	if len(enginePayload.TaskPayload.AssetIDs) == 0 || len(enginePayload.TaskPayload.BaselineAssetIDs) == 0 {
		return fmt.Errorf("Expected an array of assetIDs and baseline assetIDs provided in the payload, but instead got %d assetIDs and %d baseline assetIDs",
			len(enginePayload.TaskPayload.AssetIDs), len(enginePayload.TaskPayload.BaselineAssetIDs))
	}

	// Get the benchmark data registry ID
	if enginePayload.Test {
		fmt.Printf("For test, setting asset benchmark data registry ID: %s\n", myAppContext.Config.DataRegistryIDs.Transcription)
		benchmarkDataRegistryID = myAppContext.Config.DataRegistryIDs.Transcription
	} else {
		if enginePayload.TaskPayload.DataRegistryID == "" {
			return fmt.Errorf("[ERROR] Unable to find a data registry ID to write benchmark data to. enginePayload: %+v", enginePayload)
		}
	}

	publishedSchema, err := graphQLClient.FetchPublishedSchema(shutdownCtx, benchmarkDataRegistryID)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to fetch the schemas given the data registry ID(%s): %s", benchmarkDataRegistryID, err)
	} else if publishedSchema.DataRegistryID == "" || publishedSchema.Schema == nil || publishedSchema.Schema.ID == "" {
		return fmt.Errorf("Unable to find a published schema to use for writing benchmark data using data registry ID: %s", benchmarkDataRegistryID)
	}
	benchmarkSchemaID = publishedSchema.Schema.ID

	fmt.Printf("[InvokeService] BENCHMARK SCHEMA ID FOUND: %s\n", benchmarkSchemaID)
	if enginePayload.Debug {
		fmt.Printf("[InvokeService] [DEBUG] task payload: %+v\n", enginePayload.TaskPayload)
	}

	// Now run the main asset benchmarking logic
	err = processAssets(shutdownCtx, graphQLClient, enginePayload, benchmarkSchemaID)
	if err != nil {
		return fmt.Errorf("Failed to process assets due to: %s", err)
	}

	return nil
}

// processAssets Take a slice of assetIDs from the engine payload and run the benchmark logic.
func processAssets(shutdownCtx context.Context, graphQLClient *api.PlatformGraphQLClient, enginePayload *BenchmarkEnginePayload, benchmarkSchemaID string) error {
	assetIDs := enginePayload.TaskPayload.AssetIDs
	baselineAssetIDs := enginePayload.TaskPayload.BaselineAssetIDs

	fmt.Printf("[processAssets] Running the asset benchmark for %d assets on %d different baselines\n", len(assetIDs), len(baselineAssetIDs))

	// Gather all the assets and map them by TDOID
	// tdoAssetMap - map the TDOID to its corresponding assets and baseline asset
	// failedAssets - track the list of failed asset IDs
	tdoAssetMap, failedAssets := gatherAssetsByTDO(shutdownCtx, graphQLClient, enginePayload.TaskID, assetIDs)

	// Fetch the baseline asset for each baseline in the array and add it to the map

	// failedBaselineAssets - Track the list of failed baseline assets
	baseLineAsset, failedBaselineAssets := gatherBaselineAssets(shutdownCtx, graphQLClient, enginePayload.TaskID, tdoAssetMap, baselineAssetIDs)

	// Benchmark service endpoint
	// url := myConfig.LocalServiceURL + "/benchmark"
	// Run the benchmark service individually for each TDO ID
	for TDOID, tdoAssets := range tdoAssetMap {
		fmt.Printf("[processAssets] Benchmarking assets for TDOID %s\n", TDOID)
		if baseLineAsset == nil {
			// must have the baseline asset to perform benchmarking
			for _, asset := range tdoAssets.assets {
				failedAssets = append(failedAssets, asset.ID)
			}
			continue
		}
		var resultArray []*results
		for _, asset := range tdoAssets.assets {
			// asset.Transcript
			fmt.Println("-------------------", asset.Transcript)
			fmt.Println("+++++++++++++++++++", baseLineAsset.Transcript)
			result, err := sclite(context.Background(), true, []byte(asset.Transcript), []byte(baseLineAsset.Transcript))
			if err != nil {
				fmt.Printf("[processAssets] [WARNING] Couldn't benchmark due to: %s\n", err)
				failedAssets = append(failedAssets, asset.ID)
				continue
			}
			result.EngineID = asset.SourceData.Engine.ID
			result.AssetID = asset.ID
			result.ModelID = asset.ModelID
			result.EngineName = asset.SourceData.Engine.Name
			result.DeployedVersion = asset.SourceData.Engine.DeployedVersion
			resultArray = append(resultArray, result)
		}
		if len(resultArray) == 0 {
			for _, asset := range tdoAssets.assets {
				failedAssets = append(failedAssets, asset.ID)
			}
			continue
		}

		// Format all the asset outputs to fit the format of the local benchmark service
		// engineOutputs, newIDToEngineID := formatBenchmarkEngineOutputsPayload(tdoAssets)
		// a, _ := json.Marshal(tdoAssets)
		// fmt.Printf("--------------- %+v", tdoAssets)
		// fmt.Printf("=============== %+v", baseLineAsset.Transcript)
		// fmt.Printf("+++++++++++++++ %+v", newIDToEngineID)

		// Prepare the benchmark service payload
		// benchmarkServicePayload := BenchmarkServicePostPayload{
		// 	EngineOutputs: engineOutputs,
		// 	GroundTruth:   tdoAssets.baselineAsset.Transcript,
		// 	TdoID:         TDOID,
		// 	ScliteFQN:     "sclite",
		// }

		// sclite(myAppContext, true, []byte(engineOutputs), []byte(tdoAssets.baselineAsset.Transcript))
		// TODO: remove callBenchmarkService and use sclite instead
		// resultArray, err := callBenchmarkService(url, &benchmarkServicePayload, myConfig.LocalServiceRetry, enginePayload.Debug)
		// if err != nil {
		// 	fmt.Printf("[processAssets] [WARNING] Couldn't benchmark due to: %s\n", err)
		// 	for _, asset := range tdoAssets.assets {
		// 		failedAssets = append(failedAssets, asset.ID)
		// 	}
		// 	continue
		// } else if len(resultArray) == 0 {
		// 	fmt.Printf("[processAssets] [WARNING] No benchmarks recieved from the benchmark service!")
		// 	for _, asset := range tdoAssets.assets {
		// 		failedAssets = append(failedAssets, asset.ID)
		// 	}
		// 	continue
		// }

		for _, result := range resultArray {

			newSDO := AssetBenchmarkSDODataForTranscription{
				BenchmarkJobID:  enginePayload.JobID,
				BenchmarkTaskID: enginePayload.TaskID,
				TDOID:           TDOID,
				AssetID:         result.AssetID,
				ModelID:         result.ModelID,
				EngineID:        result.EngineID,
				OrganizationID:  enginePayload.OrganizationID,
				BaselineAssetID: tdoAssets.baselineAsset.ID,
				EngineName:      result.EngineName,
				DeployedVersion: result.DeployedVersion,
				// Metrics
				Accuracy:      result.Accuracy,
				Precision:     result.Precision,
				Recall:        result.Recall,
				WordErrorRate: result.WordErrorRate,
				Words:         result.Words,
			}

			// If a training SDO was passed, include the reference
			if enginePayload.TaskPayload.TrainingWorkflowSDOID != "" && enginePayload.TaskPayload.TrainingWorkflowSDOSchemaID != "" {
				newSDO.TrainingSDO = &SDOReference{
					ID:       enginePayload.TaskPayload.TrainingWorkflowSDOID,
					SchemaID: enginePayload.TaskPayload.TrainingWorkflowSDOSchemaID,
				}
			}

			if !enginePayload.Test {
				sdo, err := graphQLClient.CreateSDO(shutdownCtx, benchmarkSchemaID, newSDO)
				if err != nil {
					failedAssets = append(failedAssets, newSDO.AssetID)
					fmt.Printf("[processAssets] [ERROR] Error creating the benchmark SDO for asset(%s) due to: %s", newSDO.AssetID, err)
				} else {
					fmt.Printf("[processAssets] Benchmark SDO for asset(%s) successfully created with ID: %s\n", newSDO.AssetID, sdo.ID)
				}
			} else {
				fmt.Printf("[processAssets] This is a test, but the SDO would have been created...SDO: %+v\n", newSDO)
			}
		}
	}

	if len(failedAssets) > 0 || len(failedBaselineAssets) > 0 {
		fmt.Printf("[processAssets] [ERROR] Some of the assets failed to benchmark. Here is the list...\n Assets: %+v\nBaseline Assets: %+v\n", failedAssets, failedBaselineAssets)
		return fmt.Errorf("Too many assets failed to benchmark. Assets: %v, Baseline Assets: %v", failedAssets, failedBaselineAssets)
	}

	return nil
}

// gatherAssetsByTDO Gather the asset data and organize them by their corresponding TDO ID
func gatherAssetsByTDO(shutdownCtx context.Context, graphQLClient *api.PlatformGraphQLClient, taskID string, assetIDs []string) (tdoAssetMap map[string]*TDOAssets, failedAssets []string) {
	fmt.Printf("[gatherAssetsByTDO] Gathering assets from the payload and organizing them by TDO\n")
	tdoAssetMap = make(map[string]*TDOAssets)
	failedAssets = make([]string, 0)
	for _, assetID := range assetIDs {
		fmt.Printf("[gatherAssetsByTDO] Gather asset ID: %s\n", assetID)
		asset, err := graphQLClient.FetchAsset(shutdownCtx, assetID)
		if err != nil {
			// Skip the asset if there is any failure, and add it the list of failed assets
			failedAssets = append(failedAssets, assetID)
			fmt.Printf("[gatherAssetsByTDO] [WARNING] Failed to fetch asset(%s) due to: %s\n", assetID, err)
			err := graphQLClient.AppendWarningToTask(shutdownCtx, taskID, assetID, "asset_unavailable", fmt.Sprintf("Could not fetch %s to benchmark.", assetID))
			if err != nil {
				fmt.Printf("[gatherAssetsByTDO] [WARNING] Failed to update the running task with a warning about a failed asset")
			}
			continue
		}

		// Format the asset into something usable by the engine
		asset, err = compileAsset(asset)
		if err != nil {
			failedAssets = append(failedAssets, assetID)
			fmt.Printf("[gatherAssetsByTDO] [WARNING] Error compiling the asset(%s) due to: %s\n", assetID, err)
			err := graphQLClient.AppendWarningToTask(shutdownCtx, taskID, assetID, "invalid_transcript_asset", fmt.Sprintf("%s is not a valid VTN-standard transcript.", assetID))
			if err != nil {
				fmt.Printf("[gatherAssetsByTDO] [WARNING] Failed to update the running task about a failed asset due to: %s", err)
			}
			continue
		} else if asset.Container.ID == "" {
			// For some reason this asset does not have a TDOID, so fail this asset
			failedAssets = append(failedAssets, assetID)
			fmt.Printf("[gatherAssetsByTDO] [WARNING] Error compiling the asset(%s) because it did not have a TDO ID associated with it\n", assetID)
			err := graphQLClient.AppendWarningToTask(shutdownCtx, taskID, assetID, "invalid_transcript_asset", fmt.Sprintf("%s did not have a TDO ID associated with it.", assetID))
			if err != nil {
				fmt.Printf("[gatherAssetsByTDO] [WARNING] Failed to update the running task about a failed asset due to: %s", err)
			}
			continue
		}

		// Initial asset for TDO case
		if _, ok := tdoAssetMap[asset.Container.ID]; !ok {
			tdoAssetMap[asset.Container.ID] = &TDOAssets{}
		}

		// Map by the TDO ID
		assets := tdoAssetMap[asset.Container.ID].assets
		assets = append(assets, asset)
		tdoAssetMap[asset.Container.ID].assets = assets
	}

	return tdoAssetMap, failedAssets
}

// gatherBaselineAssets Gather the baseline asset data and add them to the tdoAssetMap according to its corresponding TDOID
func gatherBaselineAssets(shutdownCtx context.Context, graphQLClient *api.PlatformGraphQLClient, taskID string, tdoAssetMap map[string]*TDOAssets, baselineAssetIDs []string) (*api.Asset, string) {
	fmt.Printf("[gatherBaselineAssets] Gathering baseline assets from the payload and organizing them by TDO\n")
	var failedBaselineAssets string
	baselineAssetID := baselineAssetIDs[0]
	baselineAsset, err := graphQLClient.FetchAsset(shutdownCtx, baselineAssetID)
	if err != nil {
		failedBaselineAssets = baselineAssetID
		fmt.Printf("[gatherBaselineAssets] [WARNING] Failed to fetch the baseline asset for assetID(%s) due to: %s", baselineAssetID, err)
		err := graphQLClient.AppendWarningToTask(shutdownCtx, taskID, baselineAssetID, "asset_unavailable", fmt.Sprintf("Could not fetch baseline asset %s to benchmark.", baselineAssetID))
		if err != nil {
			fmt.Printf("[gatherBaselineAssets] [WARNING] Failed to update the running task about a failed asset due to: %s", err)
		}
	}

	// If the TDO asset map doesn't have the TDO associated with this baseline, then that means no assets were gathered in the previous step. Therefore, we should fail this baseline asset.
	if _, ok := tdoAssetMap[baselineAsset.Container.ID]; !ok {
		failedBaselineAssets = baselineAssetID
		fmt.Printf("[gatherBaselineAssets] [WARNING] The baseline asset(%s) has no other assets to benchmark against\n", baselineAssetID)
		err := graphQLClient.AppendWarningToTask(shutdownCtx, taskID, baselineAssetID, "asset_unavailable", fmt.Sprintf("Baseline asset %s has no other assets to benchmark against.", baselineAssetID))
		if err != nil {
			fmt.Printf("[gatherBaselineAssets] [WARNING] Failed to update the running task about a failed asset due to: %s", err)
		}
	}

	// Compile the raw transcript and find the model ID if it exists
	baselineAsset, err = compileAsset(baselineAsset)
	if err != nil {
		failedBaselineAssets = baselineAssetID
		fmt.Printf("[gatherBaselineAssets] [WARNING] Failed to compile baseline asset(%s) due to: %s\n", baselineAssetID, err)
		err := graphQLClient.AppendWarningToTask(shutdownCtx, taskID, baselineAssetID, "invalid_transcript_asset", fmt.Sprintf("Baseline %s is not a valid VTN-standard transcript.", baselineAssetID))
		if err != nil {
			fmt.Printf("[gatherBaselineAssets] [WARNING] Failed to update the running task about a failed asset due to: %s", err)
		}
	}

	return baselineAsset, failedBaselineAssets
}

// compileAsset Compile the provided asset to have the required VTN-standard output as a Golang struct and a string transcript.
// Also get the model ID from the asset if it exists
func compileAsset(asset *api.Asset) (*api.Asset, error) {
	fmt.Printf("[compileAsset] Compiling the asset for asset ID: %s\n", asset.ID)
	// need to convert asset transform(transformFunction: JSON) which is a string, to EngineOutput (VTN-standard)
	var output *api.EngineOutput
	err := json.Unmarshal([]byte(asset.Raw), &output)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling the asset(%s) data due to: %s", asset.ID, err)
	}
	// store the asset as output
	asset.Data = output

	if asset.SourceData.Engine == nil {
		asset.SourceData.Engine = &api.Engine{}
	}

	// convert to a string transcript
	if asset.SourceData.Engine.CategoryID == "" || asset.SourceData.Engine.CategoryID == categoryTranscriptionID {
		var transcript string
		for _, serie := range output.Series {
			for _, word := range serie.Words {
				matched, err := regexp.MatchString("!?.,:;", word.Word)
				if err == nil && matched {
					transcript = transcript + word.Word
				} else {
					transcript = transcript + " " + word.Word
				}
				transcript = strings.TrimSpace(transcript)
			}
		}
		// store the transcript as part of the asset (because it is)
		asset.Transcript = transcript
	} else {
		if asset.SourceData.Engine.CategoryID == categoryFacialDetectionID {
			for i := 0; i < len(output.Series); i++ {
				output.Series[i].Object.Rectangle = getRectangleFromPoints(output.Series[i].Object.PoundingPoly)
			}
		}
	}

	// check if the asset has modelID and add it if so (needed for SRC training workflow)
	for _, tag := range asset.Data.Tags {
		if tag.Key == "modelId" {
			asset.ModelID = tag.Value
		}
	}

	return asset, nil
}

func getRectangleFromPoints(points []api.Point) api.Rectangle {
	if len(points) < 4 {
		lenArr := 4 - len(points)
		for i := 0; i < lenArr; i++ {
			points = append(points, api.Point{
				X: 0,
				Y: 0,
			})
		}
	}

	// get Top left - Bottom right
	tl := points[0]
	br := points[0]
	for i := 1; i < 4; i++ {
		point := points[i]

		// TL
		if point.Y > tl.Y && point.X <= tl.X {
			tl = point
		}

		// BR
		if point.X > br.X && point.Y <= br.Y {
			br = point
		}
	}

	// New Rectangle
	rectangle := api.Rectangle{TL: tl, BR: br}
	rectangle.Width = math.Abs(rectangle.BR.X - rectangle.TL.X)
	rectangle.Height = math.Abs(rectangle.TL.Y - rectangle.BR.Y)
	rectangle.S = rectangle.Width * rectangle.Height

	return rectangle
}

// formatBenchmarkEngineOutputsPayload compiles the data into a format accepted by the benchmark service.
// Generates new UUIDs for each asset to cover assets of the same engine ID
func formatBenchmarkEngineOutputsPayload(tdoAssets *TDOAssets) (map[string]EngineOutput, map[string]string) {
	engineOutputs := make(map[string]EngineOutput)

	// NOTE: shouldn't map by engine ID since the client may want to benchmark two assets from the same engine,
	// so we generate a unique ID based on the asset
	newIDToEngineID := make(map[string]string)

	for _, asset := range tdoAssets.assets {
		fmt.Printf("[formatBenchmarkEngineOutputsPayload] Formatting the local service payload on asset(%s)\n", asset.ID)
		newID := uuid.New().String()
		newIDToEngineID[newID] = asset.SourceData.Engine.ID

		engineOutputs[newID] = EngineOutput{
			Output:          asset.Transcript,
			EngineName:      asset.SourceData.Engine.Name,
			AssetID:         asset.ID,
			ModelID:         asset.ModelID,
			DeployedVersion: asset.SourceData.Engine.DeployedVersion,
		}
	}

	return engineOutputs, newIDToEngineID
}

// stringInSlice returns if the provided string is in the slice
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func toJSONString(data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("%+v", data)
	}
	return string(b)
}

func sclite(ctx context.Context, includeWordBreakdown bool, ref, hyp []byte) (*results, error) {
	fileRef, err := savefile(ref)
	if err != nil {
		return nil, err
	}
	defer os.Remove(fileRef)
	refHyp, err := savefile(hyp)
	if err != nil {
		return nil, err
	}
	defer os.Remove(refHyp)
	cmd := exec.CommandContext(ctx, "/app/sclite", "-r", fileRef, "-h", refHyp, "-i", "rm", "-p")
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "cmd.StdoutPipe")
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "start")
	}
	var debug bool
	if os.Getenv("DEBUG") != "" {
		debug = true
	}
	var scanning bool
	var r results
	s := bufio.NewScanner(stdout)
	for s.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, ctx.Err()
		}
		line := s.Text()
		if debug {
			fmt.Println(line)
		}
		if strings.HasPrefix(line, "<PATH") {
			segs := strings.Split(line, `word_cnt="`)
			if len(segs) == 1 {
				return nil, errors.New("malformed XML when getting word_cnt")
			}
			r.WordCount, err = strconv.Atoi(strings.Split(segs[1], `"`)[0])
			if err != nil {
				return nil, errors.Wrap(err, "malformed XML when getting word_cnt")
			}
			scanning = true
			continue
		}
		if !scanning {
			continue
		}
		switch line {
		case "C":
			r.Correct++
		case "S":
			r.Substituted++
		case "D":
			r.Deleted++
		case "I":
			r.Inserted++
		}
		if line == "</PATH>" {
			break
		}
		w := word{
			Action: line,
		}
		if !s.Scan() {
			break
		}
		w.Reference = strings.Replace(s.Text(), `"`, ``, -1)
		if !s.Scan() {
			break
		}
		w.Hypothesis = strings.Replace(s.Text(), `"`, ``, -1)
		if includeWordBreakdown {
			r.Words = append(r.Words, w)
		}
	}
	if err := s.Err(); err != nil {
		return nil, errors.Wrap(err, "reading stdout")
	}
	if err := cmd.Wait(); err != nil {
		return nil, errors.Wrap(err, "wait")
	}
	if r.WordCount == 0 {
		return &r, nil
	}
	r.Accuracy = float64(r.Correct) / float64(r.WordCount)
	recallDenom := float64(r.Correct) + float64(r.Deleted)
	if recallDenom > 0 {
		r.Recall = float64(r.Correct) / recallDenom
	}
	precisionDenom := float64(r.Correct) + float64(r.Substituted) + float64(r.Inserted)
	if precisionDenom > 0 {
		r.Precision = float64(r.Correct) / precisionDenom
	}
	errorRateDenom := float64(r.Correct) + float64(r.Substituted) + float64(r.Deleted)
	if errorRateDenom > 0 {
		numer := float64(r.Deleted) + float64(r.Substituted) + float64(r.Inserted)
		r.WordErrorRate = numer / errorRateDenom
	}
	return &r, nil
}

func savefile(b []byte) (string, error) {
	f, err := ioutil.TempFile("", "sclite-*")
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = io.Copy(f, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	io.WriteString(f, "\n")
	return f.Name(), nil
}

var replacer = strings.NewReplacer(
	"\n", " ",
	"\r", "",
	"\t", " ",
	"|", " ",
	":", " ",
	"'", "",
	".", "",
	";", "",
	",", "",
	"?", "",
	"¿", "",
	"¡", "",
	"!", "",
	"(", "",
	")", "",
	"-", "",
	"á", "a",
	"é", "e",
	"í", "i",
	"ó", "o",
	"ú", "u",
)

func sanitize(s string) string {
	var out strings.Builder
	s = replacer.Replace(s)
	for _, field := range strings.Fields(s) {
		if strings.HasPrefix(field, "[") && strings.HasSuffix(field, "]") {
			continue
		}
		// err ignored becuase it cannot happen
		out.WriteString(field)
		out.WriteString(" ")
	}
	return out.String()
}
