# Cibles minimales (le Makefile Kubebuilder du prototype v0 a été retiré avec le Go v0).
.PHONY: build vet fmt test deadcode
build:
	go build -o bin/sylvatwin-controller ./cmd/sylvatwin-controller
	go build -o bin/sylvatwin-once ./cmd/sylvatwin-once
vet:
	go vet ./...
fmt:
	go fmt ./...
# Variables : SYLVATWIN_POC, OXIGRAPH_BIN, KUBEBUILDER_ASSETS (voir README)
test: vet
	go test ./... -skip 'TestDisqueA4|TestMesureA4'
deadcode:
	deadcode -test ./...
