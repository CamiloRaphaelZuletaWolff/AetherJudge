# executor image. Build context is backend/ (shared pkg module via replace):
#   docker build -t arena-executor -f services/executor/Dockerfile backend/
#
# The image also carries the sandbox Dockerfiles (/sandbox-images) so a
# cluster init step can build them against a pod-local Docker-in-Docker
# daemon — see infra/helm/arena/templates/executor.yaml and ADR-0009.
FROM golang:1.26-alpine AS build

WORKDIR /src

COPY pkg/go.mod pkg/go.sum pkg/
COPY services/executor/go.mod services/executor/go.sum services/executor/
WORKDIR /src/services/executor
RUN go mod download

COPY pkg/ /src/pkg/
COPY services/executor/ /src/services/executor/

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.22

# Pull the latest patched OS packages (the base tag lags behind security
# fixes — e.g. openssl libcrypto3/libssl3), then create the non-root user.
RUN apk --no-cache upgrade && adduser -D -u 10001 -s /sbin/nologin app

COPY --from=build /out/server /server
COPY services/executor/images/ /sandbox-images/

USER app
ENTRYPOINT ["/server"]
