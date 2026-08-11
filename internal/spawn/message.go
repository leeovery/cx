package spawn

import (
	"fmt"
	"strings"
)

// QuoteJoin single-quotes each name and joins them with ", ". An empty slice
// renders the empty string.
func QuoteJoin(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = "'" + name + "'"
	}
	return strings.Join(quoted, ", ")
}

// GoneVerb is the count-aware verb for the gone-session message: "is" for
// exactly one name, "are" for any other count.
func GoneVerb(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

// GoneMessage renders the pre-flight gone-session outcome sentence. The body
// carries no component prefix and no glyph; callers add those.
func GoneMessage(names []string) string {
	return fmt.Sprintf("%s %s gone — nothing opened", QuoteJoin(names), GoneVerb(len(names)))
}

// PartialFailureMessage renders the leave-what-opened outcome sentence.
// othersOpened selects the genuine-partial wording over the total-failure one.
func PartialFailureMessage(failed []string, othersOpened bool) string {
	if othersOpened {
		return fmt.Sprintf("%s failed to open — others left open", QuoteJoin(failed))
	}
	return fmt.Sprintf("%s failed to open — nothing opened", QuoteJoin(failed))
}

// UnsupportedNoopMessage renders the unsupported-terminal no-op sentence: a
// remote-connection wording for a NULL identity, otherwise one naming the
// terminal and its bundle id (the terminals.json key the user needs).
func UnsupportedNoopMessage(id Identity) string {
	if id.IsNull() {
		return "can't open new windows over a remote connection — nothing opened"
	}
	return fmt.Sprintf("can't open new windows in %s · %s — nothing opened", id.Name, id.BundleID)
}
