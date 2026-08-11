package spawn

import (
	"encoding/json"
	"errors"
	"os"
)

// Recipe expects exactly one of Argv / Script.
type Recipe struct {
	Argv   []string `json:"argv"`
	Script string   `json:"script"`
}

// Capabilities.Open is a pointer so an absent `open` sub-key decodes to nil,
// distinguishable from a present-but-empty recipe. Unmodeled keys are dropped by
// encoding/json, which is the forward-compat story for future capabilities.
type Capabilities struct {
	Open *Recipe `json:"open"`
}

type TerminalEntry struct {
	Commands Capabilities `json:"commands"`
}

// TerminalsConfig maps an identity key — whatever form the user pasted: friendly
// alias, .app name, raw bundle id or *-glob — to its entry.
type TerminalsConfig map[string]TerminalEntry

// TerminalsStore never writes, and never resolves its path — that is the cmd
// layer's job.
type TerminalsStore struct {
	path string
}

func NewTerminalsStore(path string) *TerminalsStore {
	return &TerminalsStore{path: path}
}

// Load degrades every failure to an empty non-nil config rather than an error; a
// missing file — the normal unconfigured case — does not even warn.
func (s *TerminalsStore) Load() TerminalsConfig {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TerminalsConfig{}
		}
		spawnLogger.Warn("terminals.json unreadable", "detail", err.Error())
		return TerminalsConfig{}
	}

	var cfg TerminalsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		spawnLogger.Warn("terminals.json malformed", "detail", err.Error())
		return TerminalsConfig{}
	}

	if cfg == nil {
		return TerminalsConfig{}
	}
	return cfg
}
