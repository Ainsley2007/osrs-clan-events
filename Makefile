.PHONY: run build clean test

run:
	go run cmd/osrs-events/main.go

build:
	go build -o bin/osrs-events cmd/osrs-events/main.go

clean:
	rm -rf bin/

test:
	go test ./...
