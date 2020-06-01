## Building the engine wrapper
FROM veritone/aiware-engine-toolkit as vt-engine-toolkit

FROM golang:1.12.0 as builder
ARG GITHUB_ACCESS_TOKEN
WORKDIR /
RUN apt-get update && apt-get --allow-unauthenticated install -y \
      git \
      bash

RUN git config --global url."https://${GITHUB_ACCESS_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/" && \
    go env && go list all | grep cover
ADD . /go/src/github.com/veritone/translation-benchmark
WORKDIR /go/src/github.com/veritone/translation-benchmark
RUN GOPATH=/go make go-build-all

# FROM engine-template:ubuntu as engine_template
FROM sclite as sclite
FROM ubuntu:16.04
ARG GIT_COMMIT
ARG BUILD_DATE

LABEL git.commit=$GIT_COMMIT
LABEL BUILD_DATE=$BUILD_DATE

# COPY --from=engine_template /app/engine-template /app/
COPY --from=builder /go/src/github.com/veritone/translation-benchmark/benchmark-engines-rt /app/
COPY --from=sclite /SCTK/bin/sclite /bin/sclite

RUN apt-get update && apt-get install -y \
            python3 \
            python3-dev \
            python3-pip \
            locales \
      && apt-get upgrade -y && apt-get clean && mkdir -p /app/service
RUN locale-gen en_US.UTF-8

ADD manifest.json /var/manifest.json

ADD ./service/requirements.txt /app/service/requirements.txt
RUN pip3 install --no-cache-dir -r /app/service/requirements.txt
RUN pip3 install 'pandas<0.21.0'
COPY service /app/service/

WORKDIR /app

COPY build-manifest.yml /app/
COPY config.json /app/
COPY payload_src_training.json /app/
COPY payload_assets.json /app/

ENV LANG='en_US.UTF-8' LANGUAGE='en_US:en' LC_ALL='en_US.UTF-8'
ENV ENGINE_ID 2517dfe9-b70d-43b1-bc1b-800618190d92
ENV ASSETBENCHMARKDATAREGISTRYID 219a8cc5-60fc-4c89-947a-71316bd39c75
ENV LOCAL_SERVICE_CMD "python3 /app/service/main.py --port 35000"
ENV LOCAL_SERVICE_URL "http://localhost:35000"
ENV ENGINE_CMD_LINE "/app/benchmark-engines-rt"
ENV ENGINE_WRAPPER_MANIFEST_FILE "/app/build-manifest.yml"
ENV CONFIG_FILE "/app/config.json"
ENV SERVICE_NAME "benchmark"

COPY --from=vt-engine-toolkit /opt/aiware/engine /opt/aiware/engine
ENV VERITONE_WEBHOOK_READY="http://0.0.0.0:8080/readyz"
ENV VERITONE_WEBHOOK_PROCESS="http://0.0.0.0:8080/process"

# ENTRYPOINT ["/opt/aiware/engine", "/app/engine-template"]
ENTRYPOINT ["/opt/aiware/engine", "/app/benchmark-engines-rt"]