# M3 — Dataset Layer

## Goal
Implement the dataset abstraction on top of the DB core: resolve datasets from auto-discovered tables or saved queries, execute them with pagination, filtering, and sorting.

## Depends On
M2 (DB core layer)

## Acceptance Criteria
- [ ] `Dataset` type: has a name, either a table reference or raw SQL query
- [ ] `DatasetResolver`: given a `Client`, returns all available datasets (auto-discover all tables)
- [ ] `DatasetExecutor.Query(dataset, opts)`: executes dataset with `QueryOptions` (page, pageSize, filters, sort)
- [ ] Filtering: column=value, column LIKE, column >, column < — translates to WHERE clauses safely (parameterized, no SQL injection)
- [ ] Sorting: by any column, ASC/DESC
- [ ] Pagination: LIMIT/OFFSET, returns total row count
- [ ] `QueryResult`: typed rows, column metadata, total count, current page
- [ ] Tests against real Postgres and MySQL

## Types to Implement

```go
type Dataset struct {
    Name  string
    Table string  // if auto-discovered table
    SQL   string  // if custom query
}

type QueryOptions struct {
    Page     int
    PageSize int
    Filters  []Filter
    Sort     *Sort
}

type Filter struct {
    Column   string
    Operator string // "=", "like", ">", "<", ">=" , "<="
    Value    any
}

type Sort struct {
    Column string
    Desc   bool
}

type QueryResult struct {
    Columns    []core.Column
    Rows       []map[string]any
    TotalRows  int64
    Page       int
    PageSize   int
    TotalPages int
}
```

## Notes
- Filters must use parameterized queries — never string-interpolate user input into SQL
- For custom SQL datasets, wrap as subquery: `SELECT * FROM (user_sql) AS _dc_dataset WHERE ...`
- Keep auto-discovery and custom datasets behind the same `Dataset` type — callers shouldn't care which it is

## Verify
```bash
go test ./internal/core/dataset/...
```
