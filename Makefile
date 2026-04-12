BINARY := datacow
CMD    := ./cmd

.PHONY: build test run lint clean

build:
	go build -o $(BINARY) $(CMD)

test:
	gotestsum --format testdox ./...

run:
	go run $(CMD)

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)
