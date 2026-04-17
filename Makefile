BINARY := datacow
CMD    := ./cmd

# Inside a devcontainer the DB hostnames are service names; outside they're localhost
ifneq ($(wildcard /.dockerenv),)
  POSTGRES_HOST ?= postgres
  MYSQL_HOST    ?= mysql
else
  POSTGRES_HOST ?= 127.0.0.1
  MYSQL_HOST    ?= 127.0.0.1
endif

# Inside devcontainer use uv's bundled Python (system python3 is minimal there)
# Outside fall back to whatever python3 is on PATH
ifneq ($(wildcard /.dockerenv),)
  PYTHON ?= uv run python3
else
  PYTHON ?= python3
endif

.PHONY: build test run lint clean wait-for-db preflight seed

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
	@until pg_isready -h $(POSTGRES_HOST) -U datacow -q; do sleep 1; done
	@echo "Postgres ready."
	@echo "Waiting for MySQL..."
	@until mysqladmin ping -h $(MYSQL_HOST) -u datacow -pdatacow --silent 2>/dev/null; do sleep 1; done
	@echo "MySQL ready."

seed: wait-for-db
	@echo "Generating sample SQL..."
	@PYTHONPATH= $(PYTHON) scripts/generate-sample-db.py postgres > scripts/init-postgres.sql
	@PYTHONPATH= $(PYTHON) scripts/generate-sample-db.py mysql > scripts/init-mysql.sql
	@echo "Loading Postgres sample data into datacow_sample..."
	@PGPASSWORD=datacow psql -h $(POSTGRES_HOST) -U datacow -d postgres -c "DROP DATABASE IF EXISTS datacow_sample;" 2>/dev/null; \
	 PGPASSWORD=datacow psql -h $(POSTGRES_HOST) -U datacow -d postgres -c "CREATE DATABASE datacow_sample;"
	@PGPASSWORD=datacow psql -h $(POSTGRES_HOST) -U datacow -d datacow_sample -f scripts/init-postgres.sql
	@echo "Loading MySQL sample data into datacow_sample..."
	@MYSQL_PWD=root mysql -h $(MYSQL_HOST) -u root -e "DROP DATABASE IF EXISTS datacow_sample; CREATE DATABASE datacow_sample; GRANT ALL ON datacow_sample.* TO 'datacow'@'%';"
	@MYSQL_PWD=datacow mysql -h $(MYSQL_HOST) -u datacow datacow_sample < scripts/init-mysql.sql
	@echo "Sample databases ready."

clean:
	rm -f $(BINARY)
