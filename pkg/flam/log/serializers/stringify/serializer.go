package stringify

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// newSerializer returns a stringify serializerImpl of the given id,
// formatting the timestamps with the layout.
func newSerializer(
	id string,
	format string,
) flam.ILogSerializer {
	return &serializerImpl{id: id, format: format}
}

// serializerImpl is the stringify [flam.ILogSerializer]. It holds no state
// but its id and layout, so it is safe for concurrent use and has nothing
// to close.
type serializerImpl struct {
	id     string
	format string
}

// compile time check that *serializerImpl implements ILogSerializer
var _ flam.ILogSerializer = (*serializerImpl)(nil)

func (s *serializerImpl) Id() string {
	return s.id
}

func (s *serializerImpl) Close() error {
	return nil
}

func (s *serializerImpl) Serialize(
	timestamp time.Time,
	level flam.LogLevel,
	message string,
	ctx flam.IBag,
) string {
	var b strings.Builder
	b.WriteString(timestamp.Format(s.format))
	b.WriteString(" [")
	b.WriteString(strings.ToUpper(level.String()))
	b.WriteString("] ")
	if serializerPrintable(message) && !strings.ContainsAny(message, `"=`) {
		b.WriteString(message)
	} else {
		b.WriteString(strconv.Quote(message))
	}
	if !value.IsNil(ctx) {
		serializerWriteBag(&b, "", ctx)
	}
	return b.String()
}

// serializerWriteBag writes a key=value pair per entry of the bag, in key
// order, the keys prefixed with the prefix, recursing into the nested
// bags and maps of string keys to values of any type, a nil one writing
// nothing, as an empty one.
func serializerWriteBag(
	b *strings.Builder,
	prefix string,
	bag flam.IBag,
) {
	for key, val := range bag.All() {
		if m, ok := val.(map[string]any); ok {
			val = container.Bag(m)
		}
		if nested, ok := val.(flam.IBag); ok {
			if !value.IsNil(nested) {
				serializerWriteBag(b, prefix+key+".", nested)
			}
			continue
		}
		b.WriteByte(' ')
		b.WriteString(serializerQuote(prefix + key))
		b.WriteByte('=')
		b.WriteString(serializerQuote(fmt.Sprint(val)))
	}
}

// serializerQuote returns s quoted when it is empty, or holds a space, a
// quote, an equal sign or a character that is not printable, or s
// otherwise.
func serializerQuote(
	s string,
) string {
	if s == "" || strings.ContainsAny(s, ` "=`) || !serializerPrintable(s) {
		return strconv.Quote(s)
	}
	return s
}

// serializerPrintable reports whether s holds only printable characters,
// the space included.
func serializerPrintable(
	s string,
) bool {
	return strings.IndexFunc(s, func(r rune) bool { return !unicode.IsPrint(r) }) < 0
}
