BINARY  := simupi
MODULE  := simupi
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
OUTDIR  := dist

# Detect Go binary: use $GOROOT/bin/go if 'go' is not in PATH
GO      := $(shell which go 2>/dev/null || echo /home/sreguren/go/bin/go)

.PHONY: all build linux windows darwin clean run tidy

all: linux windows darwin

## build — binario para el sistema actual
build:
	$(GO) build $(LDFLAGS) -o $(BINARY) .

## linux — amd64
linux:
	mkdir -p $(OUTDIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-linux-amd64 .

## windows — amd64
windows:
	mkdir -p $(OUTDIR)
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-windows-amd64.exe .

## darwin — arm64 (Apple Silicon) y amd64 (Intel)
darwin:
	mkdir -p $(OUTDIR)
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-darwin-amd64 .

## run — compila y ejecuta localmente
run: build
	./$(BINARY)

## tidy — actualiza go.sum y elimina dependencias no usadas
tidy:
	$(GO) mod tidy

## clean — elimina binarios generados
clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf $(OUTDIR)
