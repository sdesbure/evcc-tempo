# Build the manager binary
FROM golang:1.25.6-bookworm@sha256:52f27e9c1f64d44e8517e8e40db0476a0198647c551472e3655933a4b92e5c21 AS builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o evcc-tempo main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot@sha256:f9f84bd968430d7d35e8e6d55c40efb0b980829ec42920a49e60e65eac0d83fc
WORKDIR /
COPY --from=builder /workspace/evcc-tempo .
USER 65532:65532

ENTRYPOINT ["/evcc-tempo"]
