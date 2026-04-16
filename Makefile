BINARY := datacow
CMD    := ./cmd

.PHONY: build test run lint clean wait-for-db preflight

build:
	go build -o $(BINARY) $(CMD)

test:
	gotestsum --format testdox ./...

run:
	go run $(CMD) $(ARGS)

lint:
	golangci-lint run ./...

preflight:
	bash .devcontainer/preflight.sh

wait-for-db:
	@echo "Waiting for Postgres..."
	@until pg_isready -h postgres -U datacow -q; do sleep 1; done
	@echo "Postgres ready."
	@echo "Waiting for MySQL..."
	@until mysqladmin ping -h mysql -u datacow -pdatacow --silent 2>/dev/null; do sleep 1; done
	@echo "MySQL ready."

clean:
	rm -f $(BINARY)
