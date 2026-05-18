package views

// PageSizeRegistry tracks per-dataset page sizes for the life of the process.
// It is owned by the App and shared across all RowBrowserModel instances so that
// page-size choices survive dataset switches and drill-downs.
type PageSizeRegistry struct {
	sizes map[string]int
	def   int
}

var _ *PageSizeRegistry = (*PageSizeRegistry)(nil)

// NewPageSizeRegistry returns a registry with the given default page size.
func NewPageSizeRegistry(defaultSize int) *PageSizeRegistry {
	return &PageSizeRegistry{
		sizes: make(map[string]int),
		def:   defaultSize,
	}
}

// Get returns the page size for the named dataset, or the default if not set.
// Safe to call on a nil receiver (returns 50).
func (r *PageSizeRegistry) Get(name string) int {
	if r == nil {
		return 50
	}
	if n, ok := r.sizes[name]; ok {
		return n
	}
	return r.def
}

// Set stores the page size for the named dataset.
// Safe to call on a nil receiver (no-op).
func (r *PageSizeRegistry) Set(name string, size int) {
	if r == nil {
		return
	}
	r.sizes[name] = size
}
