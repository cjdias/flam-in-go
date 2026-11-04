package slogbridge

import (
	"context"
	"log/slog"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// HandlerOptions are the options of the handler returned by [NewHandler].
type HandlerOptions struct {
	// Channel is the channel the records are signaled on, [DefaultChannel]
	// when empty.
	Channel string

	// Level is the minimum level of the records handled,
	// [log/slog.LevelInfo] when nil. It is read on every record, so that
	// a [log/slog.LevelVar] changes it at run time.
	Level slog.Leveler

	// TimeField is the context field the time of the records is stored
	// at, when not zero. It is left out when empty, as the logger stamps
	// the entries with its own clock.
	TimeField string
}

// NewHandler returns the [log/slog.Handler] signaling the records of the
// minimum level of the options or above on their channel of the logger,
// which must not be nil, at the level mapped by [FlamLevel], with their
// message and, in the context of the entry, their attributes, and those
// given to WithAttrs, each group being a nested bag. When the logger is a
// [flam.IContextLogger], the records are signaled with the context they
// are logged with. Nil options stand for the zero options.
func NewHandler(
	logger flam.ILogger,
	opts *HandlerOptions,
) slog.Handler {
	h := &handler{logger: logger, channel: DefaultChannel, level: slog.LevelInfo}
	if opts != nil {
		if opts.Channel != "" {
			h.channel = opts.Channel
		}
		if opts.Level != nil {
			h.level = opts.Level
		}
		h.timeField = opts.TimeField
	}
	return h
}

// NewLogger returns the [log/slog.Logger] of the handler returned by
// [NewHandler] for the logger and the options.
func NewLogger(
	logger flam.ILogger,
	opts *HandlerOptions,
) *slog.Logger {
	return slog.New(NewHandler(logger, opts))
}

// handler is the [log/slog.Handler] returned by [NewHandler]. It is never
// changed once built, WithAttrs and WithGroup returning new handlers, so
// that it is safe for concurrent use.
type handler struct {
	logger    flam.ILogger
	channel   string
	level     slog.Leveler
	timeField string

	groups []string      // the groups opened by WithGroup, outermost first
	attrs  container.Bag // the attributes given to WithAttrs, nested in their groups
}

// compile time check that *handler implements Handler
var _ slog.Handler = (*handler)(nil)

func (h *handler) Enabled(
	_ context.Context,
	level slog.Level,
) bool {
	return level >= h.level.Level()
}

// Handle signals the record, with its time at the time field when set and
// the time is not zero, and its attributes nested in the groups opened by
// WithGroup, unless it has none. It always returns nil.
func (h *handler) Handle(
	ctx context.Context,
	record slog.Record,
) error {
	fields := handlerClone(h.attrs)
	if h.timeField != "" && !record.Time.IsZero() {
		fields[h.timeField] = record.Time
	}
	attrs := container.Bag{}
	record.Attrs(func(attr slog.Attr) bool {
		handlerAdd(attrs, attr)
		return true
	})
	handlerMerge(fields, h.groups, attrs)

	level := FlamLevel(record.Level)
	if logger, ok := h.logger.(flam.IContextLogger); ok {
		logger.SignalContext(ctx, level, h.channel, record.Message, fields)
		return nil
	}
	h.logger.Signal(level, h.channel, record.Message, fields)
	return nil
}

func (h *handler) WithAttrs(
	attrs []slog.Attr,
) slog.Handler {
	bag := container.Bag{}
	for _, attr := range attrs {
		handlerAdd(bag, attr)
	}
	if len(bag) == 0 {
		return h
	}
	clone := *h
	clone.attrs = handlerClone(h.attrs)
	handlerMerge(clone.attrs, h.groups, bag)
	return &clone
}

func (h *handler) WithGroup(
	name string,
) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.groups = append(h.groups[:len(h.groups):len(h.groups)], name)
	return &clone
}

// handlerClone returns a deep copy of the bag of the attributes, or an
// empty bag when nil, so that it can be changed without changing the
// handler.
func handlerClone(
	bag container.Bag,
) container.Bag {
	if bag == nil {
		return container.Bag{}
	}
	return bag.Clone().(container.Bag)
}

// handlerAdd stores the attribute in the bag, resolved, unless it is empty
// or a group without attribute, a group as a nested bag, unless its key is
// empty, its attributes being then stored in the bag.
func handlerAdd(
	bag container.Bag,
	attr slog.Attr,
) {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return
	}
	if attr.Value.Kind() != slog.KindGroup {
		bag[attr.Key] = attr.Value.Any()
		return
	}
	if attr.Key == "" {
		for _, member := range attr.Value.Group() {
			handlerAdd(bag, member)
		}
		return
	}
	group := container.Bag{}
	for _, member := range attr.Value.Group() {
		handlerAdd(group, member)
	}
	handlerMerge(bag, []string{attr.Key}, group)
}

// handlerMerge copies the fields, unless there are none, into the bag
// nested in the groups, creating them, or replacing a value that is not a
// bag. A copied field replaces the one of the same key.
func handlerMerge(
	bag container.Bag,
	groups []string,
	fields container.Bag,
) {
	if len(fields) == 0 {
		return
	}
	for _, group := range groups {
		nested, ok := bag[group].(container.Bag)
		if !ok {
			nested = container.Bag{}
			bag[group] = nested
		}
		bag = nested
	}
	for key, val := range fields {
		bag[key] = val
	}
}
