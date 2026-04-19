FROM golang:1.26 AS builder
WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.sum ./
RUN go mod download
# Copy the go source
COPY main/main.go main/main.go
# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o webserver main/main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/webserver .
USER 65532:65532
ENTRYPOINT ["/webserver"]
