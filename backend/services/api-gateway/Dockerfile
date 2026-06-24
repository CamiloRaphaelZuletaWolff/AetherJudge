# api-gateway image. Build context is backend/ (the gateway requires the
# shared pkg module via a relative replace):
#   docker build -t arena-gateway -f services/api-gateway/Dockerfile backend/
FROM golang:1.26-alpine AS build

WORKDIR /src

# Warm the module cache before copying sources so code edits don't bust it.
COPY pkg/go.mod pkg/go.sum pkg/
COPY services/api-gateway/go.mod services/api-gateway/go.sum services/api-gateway/
WORKDIR /src/services/api-gateway
RUN go mod download

COPY pkg/ /src/pkg/
COPY services/api-gateway/ /src/services/api-gateway/

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

# distroless static: no shell, no package manager, runs as nonroot (65532).
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/server /server
COPY --from=build /out/seed /seed
COPY --from=build /out/worker /worker

USER nonroot
ENTRYPOINT ["/server"]
