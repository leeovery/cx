package spawn

// Resolution's values are exactly the `resolution` log-attr vocabulary, so a
// Resolution is logged directly with no translation.
type Resolution string

const (
	ResolutionConfig Resolution = "config"
	ResolutionNative Resolution = "native"
	// ResolutionUnsupported — no adapter is available: a NULL identity, or a
	// known-but-undriven terminal.
	ResolutionUnsupported Resolution = "unsupported"
)

type AdapterResolver func(Identity) (Adapter, Resolution)

type nativeAdapter struct {
	family string
	build  func() Adapter
}

var nativeAdapters = []nativeAdapter{
	{
		family: "com.mitchellh.ghostty*",
		build:  func() Adapter { return newGhosttyAdapter() },
	},
}

type Resolver struct {
	// Config is the loaded terminals.json escape hatch. An empty or nil config
	// means the config tier never matches.
	Config TerminalsConfig
	runner recipeRunner
}

func NewResolver(cfg TerminalsConfig) *Resolver {
	return &Resolver{Config: cfg, runner: &execRecipeRunner{}}
}

// Resolve applies the precedence config override → native → unsupported. A NULL
// identity never consults the config tier, so a `*` catch-all cannot hijack it,
// and a matching but invalid config entry falls through to native rather than to
// a less-specific config entry.
func (r *Resolver) Resolve(id Identity) (Adapter, Resolution) {
	if id.IsNull() {
		return nil, ResolutionUnsupported
	}

	if adapter, ok := r.resolveConfig(id); ok {
		return adapter, ResolutionConfig
	}

	for _, entry := range nativeAdapters {
		if MatchesFamily(id.BundleID, entry.family) {
			return entry.build(), ResolutionNative
		}
	}

	return nil, ResolutionUnsupported
}

func (r *Resolver) resolveConfig(id Identity) (Adapter, bool) {
	key, entry, ok := matchConfig(r.Config, id)
	if !ok {
		return nil, false
	}

	recipe, kind, valid := validRecipeForEntry(key, entry)
	if !valid {
		return nil, false
	}

	switch kind {
	case RecipeArgv:
		return &argvRecipeAdapter{template: recipe.Argv, runner: r.runner}, true
	case RecipeScript:
		return newScriptRecipeAdapter(key, recipe.Script, r.runner)
	default:
		return nil, false
	}
}

// ResolveAdapter loads no config, so it reduces to native → unsupported.
func ResolveAdapter(id Identity) (Adapter, Resolution) {
	return NewResolver(TerminalsConfig{}).Resolve(id)
}
