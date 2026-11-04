package json

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"testing"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// creators returns the parser creators the bundle provides in the parser
// creator group.
func creators(t *testing.T) []flam.IConfigParserCreator {
	t.Helper()
	app := testutil.NewApp()
	testutil.NoError(t, json.NewBundle().Register(app))

	var result []flam.IConfigParserCreator
	testutil.NoError(t, app.Invoke(func(args struct {
		dig.In
		Creators []flam.IConfigParserCreator `group:"flam.config.parsers.creator"`
	}) {
		result = args.Creators
	}))
	return result
}

// newParser returns the parser of the id created by a factory configured
// with the defaults of the bundle and with the creators it provides.
func newParser(t *testing.T, id string) flam.IConfigParser {
	t.Helper()
	bag := container.Bag{}
	testutil.NoError(t, json.NewBundle().(flam.IConfigurableBundle).Config(bag))
	if id != json.Id {
		testutil.NoError(t, bag.Set(config.ParserConfigPath+"."+id,
			container.Bag{factory.ConfigFieldDriver: json.Driver}))
	}

	f, err := factory.New(bag.Bag(config.ParserConfigPath), creators(t)...)
	testutil.NoError(t, err)
	p, err := f.Get(id)
	testutil.NoError(t, err)
	return p
}

func parse(t *testing.T, content string) (flam.IBag, error) {
	t.Helper()
	return newParser(t, json.Id).Parse(strings.NewReader(content))
}

// failingReader fails after returning its content.
type failingReader struct {
	content io.Reader
	err     error
}

func (r *failingReader) Read(p []byte) (int, error) {
	n, err := r.content.Read(p)
	if err == io.EOF {
		return n, r.err
	}
	return n, err
}

// closeRecorder records whether it was closed.
type closeRecorder struct {
	io.Reader
	closed bool
}

func (r *closeRecorder) Close() error {
	r.closed = true
	return nil
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		testutil.Equal(t, "Id", json.Id, "json")
		testutil.Equal(t, "Driver", json.Driver, "flam.config.parsers.driver.json")
		testutil.Equal(t, "ErrorFieldID", json.ErrorFieldID, "id")
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			json.ErrRead:            "unable to read json config document",
			json.ErrInvalidDocument: "invalid json config document",
			json.ErrNotObject:       "json config document is not an object",
		} {
			testutil.Equal(t, "message", err.Error(), text)
			testutil.Equal(t, "unwrap", errors.Unwrap(err), nil)
		}
	})
}

// ---------------------------------------------------------------------------
// bundle and creator
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id and return a new bundle per call", func(t *testing.T) {
		b := json.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.config.parsers.json")
		if b == json.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure its default parser", func(t *testing.T) {
		bag := container.Bag{"flam": container.Bag{"config": container.Bag{"parsers": container.Bag{
			"other": container.Bag{factory.ConfigFieldDriver: "other"},
		}}}}
		testutil.NoError(t, json.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "parsers", bag.Get(config.ParserConfigPath), container.Bag{
			"json":  container.Bag{factory.ConfigFieldDriver: json.Driver},
			"other": container.Bag{factory.ConfigFieldDriver: "other"},
		})
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		bag := container.Bag{"flam": "scalar"}
		err := json.NewBundle().(flam.IConfigurableBundle).Config(bag)
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a single creator in the parser creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, json.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
		testutil.Equal(t, "creators", len(creators(t)), 1)
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		app := testutil.NewApp()
		boom := errors.New("boom")
		app.Fail = func(string) error { return boom }
		if err := json.NewBundle().Register(app); !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

func Test_Creator(t *testing.T) {
	c := creators(t)[0]

	t.Run("should accept only its driver", func(t *testing.T) {
		for driver, want := range map[string]bool{
			json.Driver:                       true,
			"":                                false,
			"json":                            false,
			"flam.config.parsers.driver.yaml": false,
		} {
			testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
		}
		testutil.Equal(t, "no driver", c.Accept(container.Bag{}), false)
	})

	t.Run("should create a parser with the configured id", func(t *testing.T) {
		p := newParser(t, "custom")
		testutil.Equal(t, "Id()", p.Id(), "custom")
		testutil.NoError(t, p.Close())
		testutil.NoError(t, p.Close())
	})

	t.Run("should create a new parser per call", func(t *testing.T) {
		bag := container.Bag{factory.ConfigFieldDriver: json.Driver, factory.ConfigFieldId: "a"}
		first, err := c.Create(bag)
		testutil.NoError(t, err)
		second, err := c.Create(bag)
		testutil.NoError(t, err)
		if first == second {
			t.Fatal("Create() returned the same parser twice")
		}
	})
}

// ---------------------------------------------------------------------------
// parse
// ---------------------------------------------------------------------------

func Test_Parse_Documents(t *testing.T) {
	scenarios := []struct {
		name    string
		content string
		want    flam.IBag
	}{
		{"should parse an empty object", `{}`, container.Bag{}},
		{"should give an empty bag for null", `null`, container.Bag{}},
		{"should give an empty bag for empty content", ``, container.Bag{}},
		{"should give an empty bag for blank content", " \n\t\r ", container.Bag{}},
		{"should ignore the whitespace around the object", " \n{\"a\": 1}\n ", container.Bag{"a": 1}},
		{"should parse scalars", `{"s": "x", "t": true, "f": false, "n": null}`,
			container.Bag{"s": "x", "t": true, "f": false, "n": nil}},
		{"should parse nested objects as bags", `{"a": {"b": {"c": "x"}}, "e": {}}`,
			container.Bag{"a": container.Bag{"b": container.Bag{"c": "x"}}, "e": container.Bag{}}},
		{"should parse arrays, with objects in them as bags", `{"l": [1, "x", [true], {"k": 2}, null], "e": []}`,
			container.Bag{"l": []any{1, "x", []any{true}, container.Bag{"k": 2}, nil}, "e": []any{}}},
		{"should keep the last of duplicate keys", `{"a": 1, "a": 2}`, container.Bag{"a": 2}},
		{"should keep dotted keys as they are", `{"a.b": 1}`, container.Bag{"a.b": 1}},
		{"should decode escapes and unicode", `{"s": "é\n\"", "é": "ü"}`,
			container.Bag{"s": "é\n\"", "é": "ü"}},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			bag, err := parse(t, s.content)
			testutil.NoError(t, err)
			testutil.Equal(t, "bag", bag, s.want)
		})
	}
}

func Test_Parse_Numbers(t *testing.T) {
	scenarios := []struct {
		name    string
		content string
		want    any
	}{
		{"should give an int for a whole number", `8080`, 8080},
		{"should give an int for zero", `0`, 0},
		{"should give an int for a negative whole number", `-12`, -12},
		{"should give an int for the largest int", fmt.Sprint(math.MaxInt), math.MaxInt},
		{"should give an int for the smallest int", fmt.Sprint(math.MinInt), math.MinInt},
		{"should give a float64 for a whole number beyond int", `9223372036854775808`, 9223372036854775808.0},
		{"should give a float64 for a fraction", `1.5`, 1.5},
		{"should give a float64 for a whole number written with a fraction", `1.0`, 1.0},
		{"should give a float64 for an exponent", `1e2`, 100.0},
		{"should give a float64 for a negative exponent", `-25e-1`, -2.5},
		{"should give +Inf for a number beyond float64", `1e400`, math.Inf(1)},
		{"should give -Inf for a negative number beyond float64", `-1e400`, math.Inf(-1)},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			bag, err := parse(t, `{"n": `+s.content+`}`)
			testutil.NoError(t, err)
			got := bag.Get("n")
			testutil.Equal(t, "value", got, s.want)
			if fmt.Sprintf("%T", got) != fmt.Sprintf("%T", s.want) {
				t.Fatalf("type = %T, want %T", got, s.want)
			}
		})
	}

	t.Run("should let the int getters read whole numbers", func(t *testing.T) {
		bag, err := parse(t, `{"server": {"port": 8080, "ratio": 0.5}}`)
		testutil.NoError(t, err)
		testutil.Equal(t, "Int()", bag.Int("server.port"), 8080)
		testutil.Equal(t, "Float64()", bag.Float64("server.ratio"), 0.5)
	})

	t.Run("should convert numbers inside arrays", func(t *testing.T) {
		bag, err := parse(t, `{"l": [1, 1.5, [2]]}`)
		testutil.NoError(t, err)
		testutil.Equal(t, "list", bag.Get("l"), []any{1, 1.5, []any{2}})
	})
}

func Test_Parse_Errors(t *testing.T) {
	invalid := []struct {
		name    string
		content string
	}{
		{"should refuse malformed JSON", `{"a": }`},
		{"should refuse an unterminated object", `{"a": 1`},
		{"should refuse a trailing comma", `{"a": 1,}`},
		{"should refuse comments", `{"a": 1 // c` + "\n}"},
		{"should refuse single quotes", `{'a': 1}`},
		{"should refuse a second document", `{"a": 1} {"b": 2}`},
		{"should refuse trailing data", `{"a": 1} x`},
		{"should refuse a trailing closing brace", `{"a": 1}}`},
		{"should refuse an invalid escape", `{"a": "\x"}`},
	}
	for _, s := range invalid {
		t.Run(s.name, func(t *testing.T) {
			bag, err := parse(t, s.content)
			if bag != nil {
				t.Fatal("Parse() returned a bag with an error")
			}
			if !errors.Is(err, json.ErrInvalidDocument) {
				t.Fatalf("Parse() = %v, want ErrInvalidDocument", err)
			}
			if !strings.HasPrefix(err.Error(), "unable to parse json config: invalid json config document: ") {
				t.Fatalf("Parse() error text = %q", err.Error())
			}
			testutil.Context(t, err.(*flam.Error), map[string]any{"id": "json"})
		})
	}

	for _, content := range []string{`[]`, `[{"a": 1}]`, `1`, `"x"`, `true`} {
		t.Run("should refuse a document that is not an object: "+content, func(t *testing.T) {
			bag, err := parse(t, content)
			if bag != nil {
				t.Fatal("Parse() returned a bag with an error")
			}
			testutil.Error(t, err, json.ErrNotObject,
				"unable to parse json config: json config document is not an object", map[string]any{"id": "json"})
		})
	}

	t.Run("should return the error of the reader, wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		bag, err := newParser(t, json.Id).Parse(&failingReader{strings.NewReader(`{}`), boom})
		if bag != nil {
			t.Fatal("Parse() returned a bag with an error")
		}
		testutil.Error(t, err, json.ErrRead,
			"unable to parse json config: unable to read json config document: boom", map[string]any{"id": "json"})
		if !errors.Is(err, boom) {
			t.Fatal("the error does not wrap the reader error")
		}
	})

	t.Run("should report the id of the parser", func(t *testing.T) {
		_, err := newParser(t, "custom").Parse(strings.NewReader(`[]`))
		testutil.Context(t, err.(*flam.Error), map[string]any{"id": "custom"})
	})
}

func Test_Parse_Contract(t *testing.T) {
	t.Run("should not close the reader", func(t *testing.T) {
		reader := &closeRecorder{Reader: strings.NewReader(`{}`)}
		_, err := newParser(t, json.Id).Parse(reader)
		testutil.NoError(t, err)
		testutil.Equal(t, "closed", reader.closed, false)
	})

	t.Run("should return a new bag on each call", func(t *testing.T) {
		p := newParser(t, json.Id)
		first, err := p.Parse(strings.NewReader(`{"a": {"b": 1}}`))
		testutil.NoError(t, err)
		testutil.NoError(t, first.Set("a.b", 2))
		second, err := p.Parse(strings.NewReader(`{"a": {"b": 1}}`))
		testutil.NoError(t, err)
		testutil.Equal(t, "a.b", second.Int("a.b"), 1)
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		p := newParser(t, json.Id)
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Go(func() {
				bag, err := p.Parse(strings.NewReader(fmt.Sprintf(`{"i": %d}`, i)))
				if err != nil || bag.Int("i") != i {
					t.Errorf("Parse(%d) = %v, %v", i, bag, err)
				}
			})
		}
		wg.Wait()
	})
}
