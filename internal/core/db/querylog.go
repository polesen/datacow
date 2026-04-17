package db

import (
	"sync"
	"time"
)

// QueryKind distinguishes user-initiated queries from system/introspection ones.
type QueryKind int

const (
	QueryKindUser   QueryKind = iota // triggered by user navigation
	QueryKindSystem                  // schema introspection / datacow-internal
)

// QueryEntry records a single query invocation.
type QueryEntry struct {
	ID        int
	Label     string
	SQL       string
	Kind      QueryKind
	StartedAt time.Time
	Duration  time.Duration // 0 = still running
	RowCount  int64
	Error     error
}

// QueryLog is a thread-safe ring buffer tracking in-flight and completed queries.
type QueryLog struct {
	mu      sync.RWMutex
	nextID  int
	running map[int]*QueryEntry // in-flight; Duration==0
	history []QueryEntry        // completed, newest-first, cap 200
}

// NewQueryLog returns an empty QueryLog.
func NewQueryLog() *QueryLog {
	return &QueryLog{
		running: make(map[int]*QueryEntry),
	}
}

func (l *QueryLog) begin(label, sql string, kind QueryKind) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := l.nextID
	l.nextID++
	l.running[id] = &QueryEntry{
		ID:        id,
		Label:     label,
		SQL:       sql,
		Kind:      kind,
		StartedAt: time.Now(),
	}
	return id
}

func (l *QueryLog) end(id int, rowCount int64, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.running[id]
	if !ok {
		return
	}
	delete(l.running, id)
	e.Duration = time.Since(e.StartedAt)
	e.RowCount = rowCount
	e.Error = err
	l.history = append([]QueryEntry{*e}, l.history...)
	if len(l.history) > 200 {
		l.history = l.history[:200]
	}
}

// RunningCount returns the number of in-flight queries.
func (l *QueryLog) RunningCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.running)
}

// CurrentLabel returns the label of an arbitrary in-flight query (empty if none).
func (l *QueryLog) CurrentLabel() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, e := range l.running {
		return e.Label
	}
	return ""
}

// Snapshot returns copies of all running and history entries.
// Running entries have Duration set to current elapsed time.
func (l *QueryLog) Snapshot() (running []QueryEntry, history []QueryEntry) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	now := time.Now()
	for _, e := range l.running {
		entry := *e
		entry.Duration = now.Sub(e.StartedAt)
		running = append(running, entry)
	}
	history = make([]QueryEntry, len(l.history))
	copy(history, l.history)
	return running, history
}
