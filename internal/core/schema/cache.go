package schema

import (
	"context"
	"sync"

	"github.com/sahilm/fuzzy"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/polesen/datacow/internal/core/db"
)

// EntryKind classifies a search entry by its schema object type.
type EntryKind string

const (
	EntryKindTable      EntryKind = "table"
	EntryKindView       EntryKind = "view"
	EntryKindDataset    EntryKind = "dataset"
	EntryKindColumn     EntryKind = "column"
	EntryKindDatasource EntryKind = "datasource"
)

// SearchEntry is one item in the goto index.
type SearchEntry struct {
	Kind      EntryKind
	Name      string           // display and fuzzy-match target; columns: "table.column"
	TableName string           // parent table (columns) or same as Name (others)
	Dataset   *dataset.Dataset // navigation target; nil for datasource entries
	DSName    string           // non-empty for datasource entries
}

// MatchResult is one fuzzy search hit.
type MatchResult struct {
	Entry          SearchEntry
	MatchedIndexes []int // positions in Name that matched, for highlighting
}

// Cache holds the full schema snapshot for one datasource.
type Cache struct {
	mu       sync.RWMutex
	ready    bool
	tables   []Table
	datasets []dataset.Dataset
	entries  []SearchEntry
}

// NewCache returns an empty, unloaded Cache.
func NewCache() *Cache {
	return &Cache{}
}

// Ready reports whether the cache has been loaded.
func (c *Cache) Ready() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready
}

// Tables returns the cached table schema.
func (c *Cache) Tables() []Table {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tables
}

// Datasets returns the cached datasets.
func (c *Cache) Datasets() []dataset.Dataset {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.datasets
}

// searchSource implements fuzzy.Source over []SearchEntry.
type searchSource struct {
	entries []SearchEntry
}

func (s searchSource) String(i int) string { return s.entries[i].Name }
func (s searchSource) Len() int            { return len(s.entries) }

// Search runs a fuzzy match over the entry index.
// Empty query returns all entries in default sort order.
func (c *Cache) Search(query string) []MatchResult {
	c.mu.RLock()
	entries := c.entries
	c.mu.RUnlock()

	if query == "" {
		results := make([]MatchResult, len(entries))
		for i, e := range entries {
			results[i] = MatchResult{Entry: e}
		}
		return results
	}

	matches := fuzzy.FindFrom(query, searchSource{entries})
	results := make([]MatchResult, len(matches))
	for i, m := range matches {
		results[i] = MatchResult{
			Entry:          entries[m.Index],
			MatchedIndexes: m.MatchedIndexes,
		}
	}
	return results
}

// Load populates the cache from the database. Safe to call concurrently with Search.
func (c *Cache) Load(ctx context.Context, client db.Client, resolver *dataset.Resolver) error {
	return c.load(ctx, client, resolver)
}

// Refresh re-populates the cache atomically. Safe to call concurrently with Search.
func (c *Cache) Refresh(ctx context.Context, client db.Client, resolver *dataset.Resolver) error {
	return c.load(ctx, client, resolver)
}

func (c *Cache) load(ctx context.Context, client db.Client, resolver *dataset.Resolver) error {
	tables, err := Load(ctx, client)
	if err != nil {
		return err
	}

	datasets, err := resolver.Resolve(ctx)
	if err != nil {
		return err
	}

	c.setData(tables, datasets, buildEntries(tables, datasets))
	return nil
}

func (c *Cache) setData(tables []Table, datasets []dataset.Dataset, entries []SearchEntry) {
	c.mu.Lock()
	c.tables = tables
	c.datasets = datasets
	c.entries = entries
	c.ready = true
	c.mu.Unlock()
}

// NewCacheWithData creates a pre-populated Cache from caller-supplied schema data.
// Intended for testing and the HTTP API layer.
func NewCacheWithData(tables []Table, datasets []dataset.Dataset) *Cache {
	c := NewCache()
	c.setData(tables, datasets, buildEntries(tables, datasets))
	return c
}

// buildEntries constructs the search index in default sort order:
// tables → views → datasets → columns.
func buildEntries(tables []Table, datasets []dataset.Dataset) []SearchEntry {
	// Map table name → dataset for column navigation (prefer auto-discovered entries).
	tableToDS := make(map[string]*dataset.Dataset, len(datasets))
	for i := range datasets {
		ds := &datasets[i]
		if ds.Table != "" {
			if _, exists := tableToDS[ds.Table]; !exists {
				tableToDS[ds.Table] = ds
			}
		}
	}

	var entries []SearchEntry

	// Dataset entries in order: tables → views → custom datasets.
	for _, kind := range []dataset.Kind{dataset.KindTable, dataset.KindView, dataset.KindDataset} {
		for i := range datasets {
			if datasets[i].Kind != kind {
				continue
			}
			eKind := EntryKindTable
			switch kind {
			case dataset.KindView:
				eKind = EntryKindView
			case dataset.KindDataset:
				eKind = EntryKindDataset
			}
			entries = append(entries, SearchEntry{
				Kind:      eKind,
				Name:      datasets[i].Name,
				TableName: datasets[i].Name,
				Dataset:   &datasets[i],
			})
		}
	}

	// Column entries — real DB tables/views only; YAML SQL datasets have no fixed schema.
	for i := range tables {
		t := &tables[i]
		dsPtr := tableToDS[t.Name]
		for _, col := range t.Columns {
			entries = append(entries, SearchEntry{
				Kind:      EntryKindColumn,
				Name:      t.Name + "." + col.Name,
				TableName: t.Name,
				Dataset:   dsPtr,
			})
		}
	}

	return entries
}
