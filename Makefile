.PHONY: build test lint image clean

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o server .

test:
	go test ./...

lint:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "run gofmt -w ."; exit 1; }
	go vet ./...

image:
	podman build -t telegram-alert-server .

clean:
	rm -f server
