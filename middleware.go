package eventsourcing

// NextHandler is a callback function that runs the next handler in a middleware
// chain.
type NextHandler func() error

// CommitMiddleware is middleware that handles commit operations, allowing for
// intercepting or other operations.
type CommitMiddleware func(writer StoreWriterAdapter, next NextHandler) error

// RefreshMiddleware is middleware that handles refresh/load operations, allowing
// for interception or other operations
type RefreshMiddleware func(reader StoreLoaderAdapter, next NextHandler) error

// wrapper is our wrapper type that creates a middleware enabled-store
type wrapper struct {
	commit  []CommitMiddleware  // Commit middlewares
	refresh []RefreshMiddleware // Refresh middlewares
	cleanup []func() error      // Cleanup functions
	inner   EventStore          // Event store we are wrapping
}

// NewMiddlewareWrapper is an event-store wrapper that provides the ability to
// insert middleware into the pipeline.
func NewMiddlewareWrapper(inner EventStore) EventStoreWithMiddleware {
	return &wrapper{
		cleanup: make([]func() error, 0),
		commit:  make([]CommitMiddleware, 0),
		refresh: make([]RefreshMiddleware, 0),
		inner:   inner,
	}
}

// Use a middleware (specific commit, refresh and cleanup together)
func (store *wrapper) Use(commit CommitMiddleware, refresh RefreshMiddleware, cleanup func() error) {
	store.HandleCleanup(cleanup)
	store.HandleCommit(commit)
	store.HandleRefresh(refresh)
}

// HandleCleanup registers a cleanup
func (store *wrapper) HandleCleanup(cleanup func() error) {
	if cleanup == nil {
		return
	}

	store.cleanup = append(store.cleanup, cleanup)
}

// HandleCommit appends a new middleware that handles a commit.
func (store *wrapper) HandleCommit(middleware CommitMiddleware) {
	if middleware == nil {
		return
	}

	store.commit = append(store.commit, middleware)
}

// HandleRefresh appends a new middleware that handles a refresh
func (store *wrapper) HandleRefresh(middleware RefreshMiddleware) {
	if middleware == nil {
		return
	}

	store.refresh = append(store.refresh, middleware)
}

// CommitEvents stores any events for the specified aggregate that are uncommitted
// at this point in time.
func (store *wrapper) CommitEvents(writer StoreWriterAdapter) error {
	// The first link in the chain is the base function
	chain := func() error {
		return store.inner.CommitEvents(writer)
	}

	for index := range store.commit {
		curr := index
		previous := chain
		chain = func() error {
			return store.commit[curr](writer, previous)
		}
	}

	return chain()
}

// Refresh an aggregates state
func (store *wrapper) Refresh(reader StoreLoaderAdapter) error {
	// The first link in the chain is the base function
	chain := func() error {
		return store.inner.Refresh(reader)
	}

	for index := range store.refresh {
		curr := index
		previous := chain
		chain = func() error {
			return store.refresh[curr](reader, previous)
		}
	}

	return chain()
}

// Close shuts down the the store driver
func (store *wrapper) Close() error {
	for _, c := range store.cleanup {
		errCleanup := c()
		if errCleanup != nil {
			return errCleanup
		}
	}

	return store.inner.Close()
}
