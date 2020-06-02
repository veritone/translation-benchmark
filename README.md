# translation-benchmark
- Batch engine to benchmark the provided translation engines against a ground truth
  - Details
    - Accepts multiple asset IDs to be benchmarked against multiple baseline asset IDs
      - The engine will benchmark each asset to its corresponding baseline asset ID (by TDO)
    - Benchmarks each asset against the baseline and creates a benchmark SDO per asset
    - Data registry IDs for benchmarks are 
      + Translation (need create one new): the `219a8cc5-60fc-4c89-947a-71316bd39c75` is for transcriptionn

- Build
  - This engine uses `sclite` to perform benchmarking, so to keep everything running smoothly, please add `sclite` to your path
  - `make build GITHUB_ACCESS_TOKEN=<token>`
  - The Github access token is used to retrieve the batch engine template

- Payload fields
  - `assetIds: ["<assetid1>", "<assetid2>"]`
    - A list of asset IDs that should be benchmarked against some corresponding baseline asset
    - Asset IDs must have exactly 1 corresponding baseline asset ID by TDO and engine ID
  - `baselineAssetIds: ["<baselineassetid1>", "<baselineassetid2>"]`
    - A list of baseline asset IDs that should be used as baselines for the corresponding asset IDs
    - Baseline asset IDs can have any number of corresponding asset IDs by TDO and engine ID
  - `categoryId: type: string. 3b2b2ff8-44aa-4db4-9b71-ff96c3bf5923`
    - This is a category ID for the job. The default is translation category
  - `dataRegistryId: type: string. (need create one new): the 219a8cc5-60fc-4c89-947a-71316bd39c75 is for transcriptionn`
    - This is a data registry ID for Transcription or Face detection. The default is the data registry for transcription
  - `minPrecision: number`
    - The minvalue of percent overlap between baseline and another. If it is < 0 => default 40 percent of overlap.
  - `debug: true`
    - A boolean denoting whether you want to allow more verbose logging in the engine
  - `test: true`
    - A boolean denoting that you are testing the engine (only use when testing the engine locally)

- Reference
  - https://github.com/veritone/textcompare 
  - https://github.com/veritone/task-benchmark-engines

