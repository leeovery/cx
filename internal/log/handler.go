package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

var stderrFallback io.Writer = os.Stderr

// componentKey is rendered by textHandler as a literal prefix before the colon,
// never as a key=value pair.
const componentKey = "component"

const processComponent = "process"

const bootstrapComponent = "bootstrap"

// A process-component record carrying one of these messages writes regardless of
// the configured level: they are forensic tripwires that must be present even at
// PORTAL_LOG_LEVEL=error. They remain semantically INFO.
var lifecycleBypassMsgs = map[string]bool{
	"start":              true,
	"exit":               true,
	"exec":               true,
	"panic":              true,
	"log-level resolved": true,
}

// textHandler injects the pid/version/process_role baselines per-record rather
// than via root.With, so a logger cached before this handler was constructed
// still carries them.
type textHandler struct {
	w     io.Writer
	level slog.Leveler

	pid         int
	version     string
	processRole string

	attrs       []prefixedAttr
	groupPrefix string
}

type prefixedAttr struct {
	prefix string
	attr   slog.Attr
}

func newTextHandler(w io.Writer, level slog.Leveler, pid int, version, processRole string) slog.Handler {
	return &textHandler{
		w:           w,
		level:       level,
		pid:         pid,
		version:     version,
		processRole: processRole,
	}
}

// Enabled is a coarse INFO-floor pre-gate, not the authoritative level filter.
// slog skips Handle entirely when Enabled returns false, so it must admit INFO
// even when the handler is configured at WARN/ERROR — otherwise a lifecycle
// marker would be dropped before Handle could apply its bypass.
func (h *textHandler) Enabled(_ context.Context, level slog.Level) bool {
	floor := min(h.level.Level(), slog.LevelInfo)
	return level >= floor
}

// Handle is the authoritative level filter, and always returns nil: logging owns
// no control flow, so an open or write failure is swallowed after one attempt on
// the stderr fallback rather than propagated to the caller.
func (h *textHandler) Handle(_ context.Context, r slog.Record) error {
	component := h.component(r)
	bypass := component == processComponent && lifecycleBypassMsgs[r.Message]
	if !bypass && r.Level < h.level.Level() {
		return nil
	}

	h.bestEffortWrite(h.render(r))
	return nil
}

func (h *textHandler) render(r slog.Record) string {
	var b strings.Builder

	b.WriteString(r.Time.Format(time.RFC3339Nano))
	b.WriteByte(' ')
	b.WriteString(r.Level.String())
	b.WriteByte(' ')
	b.WriteString(h.component(r))
	b.WriteString(": ")
	b.WriteString(r.Message)

	for _, pa := range h.attrs {
		if pa.prefix == "" && pa.attr.Key == componentKey {
			continue
		}
		writeAttr(&b, pa.prefix, pa.attr)
	}
	r.Attrs(func(a slog.Attr) bool {
		if h.groupPrefix == "" && a.Key == componentKey {
			return true
		}
		writeAttr(&b, h.groupPrefix, a)
		return true
	})

	b.WriteString(" pid=")
	b.WriteString(strconv.Itoa(h.pid))
	b.WriteString(" version=")
	b.WriteString(quoteIfMultiWord(h.version))
	b.WriteString(" process_role=")
	b.WriteString(quoteIfMultiWord(h.processRole))

	b.WriteByte('\n')

	return b.String()
}

func (h *textHandler) bestEffortWrite(line string) {
	if _, err := io.WriteString(h.w, line); err != nil {
		_, _ = fmt.Fprint(stderrFallback, line)
	}
}

func (h *textHandler) component(r slog.Record) string {
	var found string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == componentKey {
			found = a.Value.Resolve().String()
			return false
		}
		return true
	})
	if found != "" {
		return found
	}
	for _, pa := range h.attrs {
		if pa.prefix == "" && pa.attr.Key == componentKey {
			return pa.attr.Value.Resolve().String()
		}
	}
	return found
}

func (h *textHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := h.clone()
	for _, a := range attrs {
		clone.attrs = append(clone.attrs, prefixedAttr{prefix: h.groupPrefix, attr: a})
	}
	return clone
}

func (h *textHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := h.clone()
	clone.groupPrefix = h.groupPrefix + name + "."
	return clone
}

func (h *textHandler) clone() *textHandler {
	attrs := make([]prefixedAttr, len(h.attrs))
	copy(attrs, h.attrs)
	c := *h
	c.attrs = attrs
	return &c
}

func writeAttr(b *strings.Builder, prefix string, a slog.Attr) {
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		group := v.Group()
		if len(group) == 0 {
			return
		}
		// An empty group key is inlined, with no dotted segment, per slog semantics.
		nested := prefix
		if a.Key != "" {
			nested = prefix + a.Key + "."
		}
		for _, ga := range group {
			writeAttr(b, nested, ga)
		}
		return
	}
	if a.Key == "" {
		return
	}
	b.WriteByte(' ')
	b.WriteString(prefix)
	b.WriteString(a.Key)
	b.WriteByte('=')
	b.WriteString(formatValue(v))
}

func formatValue(v slog.Value) string {
	if v.Kind() == slog.KindDuration {
		return v.Duration().String()
	}
	if v.Kind() == slog.KindString {
		return quoteIfMultiWord(v.String())
	}
	return v.String()
}

func quoteIfMultiWord(s string) string {
	if strings.ContainsFunc(s, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }) {
		return `"` + s + `"`
	}
	return s
}
