BINARY := datacow
CMD    := ./cmd

.PHONY: build test run lint clean

build:
	go build -o $(BINARY) $(CMD)

test:
	go test ./...

run:
	go run $(CMD)

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)
