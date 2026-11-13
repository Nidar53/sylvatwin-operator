# Contrôleur SylvaTwin (cmd/sylvatwin-controller)
FROM golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o sylvatwin-controller ./cmd/sylvatwin-controller

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/sylvatwin-controller .
USER 65532:65532
ENTRYPOINT ["/sylvatwin-controller"]
