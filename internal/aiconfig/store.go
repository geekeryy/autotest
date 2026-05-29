package aiconfig

import "sync"

var globalStore = NewStore()

// SetGlobalStore replaces the process-wide settings store (mainly for tests).
func SetGlobalStore(s *Store) {
	if s == nil {
		globalStore = NewStore()
		return
	}
	globalStore = s
}

// Get returns the effective AI assistant configuration.
func Get() Config {
	return globalStore.Get()
}

// Store holds the in-memory effective config, refreshed from DB or updates.
type Store struct {
	mu  sync.RWMutex
	cfg Config
}

// NewStore creates a store seeded with defaults merged from env.
func NewStore() *Store {
	cfg := MergeEnv(DefaultConfig())
	return &Store{cfg: cfg}
}

// Get returns a copy of the current config.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Set replaces the entire effective config.
func (s *Store) Set(cfg Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
}

// LoadFromStored rebuilds effective config: defaults → env → stored JSON.
func (s *Store) LoadFromStored(raw storedConfig) {
	cfg := ApplyStored(MergeEnv(DefaultConfig()), raw)
	s.Set(cfg)
}
