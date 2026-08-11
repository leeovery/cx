package spawn

// Resolution classifies how an Identity mapped to an Adapter, or to no adapter.
// Its values are exactly the `resolution` log-attr vocabulary, so a Resolution
// is logged directly with no translation.
type Resolution string

const (
	ResolutionConfig Resolution = "config"
	ResolutionNative Resolution = "native"
	// ResolutionUnsupported — no adapter is available: a NULL identity, or a
	// known-but-undriven terminal.
	ResolutionUnsupported Resolution = "unsupported"
)

// AdapterResolver maps a detected host Identity to the Adapter that opens
// windows for it, plus the Resolution classifying how the mapping was made.
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

// Resolver maps a host-terminal Identity to an Adapter, applying the precedence
// config override → native adapter → unsupported.
type Resolver struct {
	// Config is the loaded terminals.json escape hatch. An empty or nil config
	// means the config tier never matches.
	Config TerminalsConfig
	runner recipeRunner
}

// NewResolver returns a config-aware Resolver over cfg, wired to the production
// recipe runner.
func NewResolver(cfg TerminalsConfig) *Resolver {
	return &Resolver{Config: cfg, runner: &execRecipeRunner{}}
}

// Resolve maps an Identity to its Adapter plus the Resolution describing how,
// with precedence config override → native → unsupported. A NULL identity
// resolves to unsupported without consulting the config tier, so a `*` catch-all
// entry cannot hijack it. A matching but invalid config entry falls through to
// the native tier, never to a less-specific config entry.
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

// ResolveAdapter resolves id with no config loaded, so behaviour reduces to
// native → unsupported.
func ResolveAdapter(id Identity) (Adapter, Resolution) {
	return NewResolver(TerminalsConfig{}).Resolve(id)
}
