# Fix: context.Context Not First Parameter in cache_test.go

`setupCacheTables` in `internal/core/schema/cache_test.go` has `ctx context.Context` as the third parameter, after `client db.Client`. The Go convention (from [Code Review Comments](https://go.dev/wiki/CodeReviewComments#contexts)) is that `context.Context` must always be the first parameter of any function that accepts one — except for `t *testing.T` which by its own convention leads test helpers.

## Current (`cache_test.go:308`)

```go
func setupCacheTables(t *testing.T, client db.Client, ctx context.Context) {
```

Called at lines 41, 56, and 272:
```go
setupCacheTables(t, client, ctx)
```

## Fix

Reorder the parameters so `ctx` comes immediately after `t`:

```go
func setupCacheTables(t *testing.T, ctx context.Context, client db.Client) {
```

Update all three call sites to match:
```go
setupCacheTables(t, ctx, client)
```

No logic changes — parameter reorder only.

## Files to modify

| File | Lines |
|---|---|
| `internal/core/schema/cache_test.go` | Line 308 (signature), lines 41, 56, 272 (call sites) |

## Definition of Done

```bash
make preflight
go build ./...
gotestsum --format testdox ./...
staticcheck ./...
make lint
```
