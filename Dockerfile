ARG GO_VERSION=1.24

FROM golang:${GO_VERSION}-bookworm AS base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

FROM base AS build
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ENV CGO_ENABLED=0
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /out/petstore-api ./cmd/api
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /out/petstore-worker ./cmd/worker

FROM gcr.io/distroless/static:nonroot AS worker
WORKDIR /app
ENV GIN_MODE=release
COPY --from=build /out/petstore-worker /app/petstore-worker
ENTRYPOINT ["/app/petstore-worker"]

FROM gcr.io/distroless/static:nonroot AS api
WORKDIR /app
ENV GIN_MODE=release
COPY --from=build /out/petstore-api /app/petstore-api
COPY --from=build /src/api /app/api
EXPOSE 8080
ENTRYPOINT ["/app/petstore-api"]
