package middlewares

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	flamenvelope "github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/envelope"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// internalBody is the problem answering an unexpected panic of the
// endpoint 7 of the service 3 with envelope.ErrInternal.
const internalBody = `{"detail":"internal server error","id":"s:3.e:7","status":500,` +
	`"title":"Internal Server Error","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/500"}`

// createEnvelope returns the envelope middleware "main" of the fields,
// created with the envelope bundle, the values and the logger when not
// nil.
func createEnvelope(t *testing.T, fields container.Bag, values map[string]any, logger flam.ILogger) (flam.IRestMiddleware, error) {
	t.Helper()
	return create(t, envelope.NewBundle(), envelope.Driver, fields, values, deps{logger: logger})
}

// assertPanicEntry asserts that the entry reports the panic with the value
// of a handler of the endpoint 7 of the pattern "GET /items/{id}", on the
// default channel at the default level.
func assertPanicEntry(t *testing.T, got entry, value any) {
	t.Helper()
	testutil.Equal(t, "level", got.level, flam.LogError)
	testutil.Equal(t, "channel", got.channel, "flam.rest")
	testutil.Equal(t, "message", got.message, "rest handler panicked")
	testutil.Equal(t, "endpoint", got.ctx.Get(envelope.ContextFieldEndpoint), any(7))
	testutil.Equal(t, "pattern", got.ctx.Get(envelope.ContextFieldPattern), any("GET /items/{id}"))
	testutil.Equal(t, "panic", got.ctx.Get(envelope.ContextFieldPanic), value)
	stack, _ := got.ctx.Get(envelope.ContextFieldStack).(string)
	testutil.Equal(t, "stack", strings.HasPrefix(stack, "goroutine "), true)
	testutil.Equal(t, "fields", len(got.ctx.(container.Bag)), 4)
}

func Test_Envelope(t *testing.T) {
	t.Run("should serve the requests without panic unchanged", func(t *testing.T) {
		logger := &entries{}
		m, err := createEnvelope(t, container.Bag{}, nil, logger)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", m.Id(), "main")

		rec, panicked := serve(t, "/items/1", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("ok"))
		}, m)
		testutil.Equal(t, "panicked", panicked, nil)
		testutil.Equal(t, "status", rec.Code, http.StatusAccepted)
		testutil.Equal(t, "body", rec.Body.String(), "ok")
		testutil.Equal(t, "entries", len(logger.all()), 0)
	})

	t.Run("should answer an envelope panic with it, unreported", func(t *testing.T) {
		for name, tc := range map[string]struct {
			value       flamenvelope.Envelope
			status      int
			contentType string
			body        string
		}{
			"data": {
				value:       flamenvelope.NewEnvelope(map[string]any{"name": "item"}),
				status:      http.StatusOK,
				contentType: "application/json",
				body:        `{"data":{"name":"item"}}`,
			},
			"error": {
				value: flamenvelope.NewEnvelope().
					WithNewError(http.StatusConflict, errors.New("item locked"), map[string]any{"item": 1}),
				status:      http.StatusConflict,
				contentType: flamenvelope.ContentTypeProblem,
				body: `{"detail":"item locked","id":"s:3.e:7","item":1,"status":409,` +
					`"title":"Conflict","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/409"}`,
			},
		} {
			t.Run(name, func(t *testing.T) {
				logger := &entries{}
				m, err := createEnvelope(t, container.Bag{}, nil, logger)
				testutil.NoError(t, err)

				rec, panicked := serve(t, "/items/1", func(http.ResponseWriter, *http.Request) { panic(tc.value) }, m)
				testutil.Equal(t, "panicked", panicked, nil)
				testutil.Equal(t, "status", rec.Code, tc.status)
				testutil.Equal(t, "content type", rec.Header().Get("Content-Type"), tc.contentType)
				testutil.Equal(t, "body", rec.Body.String(), tc.body)
				testutil.Equal(t, "entries", len(logger.all()), 0)
			})
		}
	})

	t.Run("should answer an envelope error panic with an envelope of it, unreported", func(t *testing.T) {
		logger := &entries{}
		m, err := createEnvelope(t, container.Bag{}, nil, logger)
		testutil.NoError(t, err)

		value := flamenvelope.NewError(http.StatusNotFound, errors.New("item not found")).WithErrorId(12)
		rec, panicked := serve(t, "/items/1", func(http.ResponseWriter, *http.Request) { panic(value) }, m)
		testutil.Equal(t, "panicked", panicked, nil)
		testutil.Equal(t, "status", rec.Code, http.StatusNotFound)
		testutil.Equal(t, "content type", rec.Header().Get("Content-Type"), flamenvelope.ContentTypeProblem)
		testutil.Equal(t, "body", rec.Body.String(), `{"detail":"item not found","id":"s:3.e:7.c:12","status":404,`+
			`"title":"Not Found","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/404"}`)
		testutil.Equal(t, "entries", len(logger.all()), 0)
	})

	t.Run("should answer an unexpected panic with a 500 envelope and report it", func(t *testing.T) {
		for name, value := range map[string]any{
			"plain value":          "secret detail",
			"error value":          fmt.Errorf("secret detail: %w", errBoom),
			"envelope error value": &flamenvelope.Error{Status: http.StatusNotFound, Context: container.Bag{flamenvelope.ContextFieldMessage: "secret detail"}},
		} {
			t.Run(name, func(t *testing.T) {
				logger := &entries{}
				m, err := createEnvelope(t, container.Bag{}, nil, logger)
				testutil.NoError(t, err)

				rec, panicked := serve(t, "/items/1", func(http.ResponseWriter, *http.Request) { panic(value) }, m)
				testutil.Equal(t, "panicked", panicked, nil)
				testutil.Equal(t, "status", rec.Code, http.StatusInternalServerError)
				testutil.Equal(t, "content type", rec.Header().Get("Content-Type"), flamenvelope.ContentTypeProblem)
				testutil.Equal(t, "body", rec.Body.String(), internalBody)
				testutil.Equal(t, "leak", strings.Contains(rec.Body.String(), "secret"), false)

				got := logger.all()
				testutil.Equal(t, "count", len(got), 1)
				assertPanicEntry(t, got[0], value)
			})
		}
	})

	t.Run("should expose the message of an error panic only when configured", func(t *testing.T) {
		for name, tc := range map[string]struct {
			value any
			body  string
		}{
			"error value": {
				value: fmt.Errorf("secret detail: %w", errBoom),
				body: `{"detail":"secret detail: boom","id":"s:3.e:7","status":500,` +
					`"title":"Internal Server Error","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/500"}`,
			},
			"plain value": {
				value: "secret detail",
				body:  internalBody,
			},
		} {
			t.Run(name, func(t *testing.T) {
				logger := &entries{}
				m, err := createEnvelope(t, container.Bag{envelope.ConfigFieldExposeErrors: true}, nil, logger)
				testutil.NoError(t, err)

				rec, panicked := serve(t, "/items/1", func(http.ResponseWriter, *http.Request) { panic(tc.value) }, m)
				testutil.Equal(t, "panicked", panicked, nil)
				testutil.Equal(t, "status", rec.Code, http.StatusInternalServerError)
				testutil.Equal(t, "content type", rec.Header().Get("Content-Type"), flamenvelope.ContentTypeProblem)
				testutil.Equal(t, "body", rec.Body.String(), tc.body)

				got := logger.all()
				testutil.Equal(t, "count", len(got), 1)
				assertPanicEntry(t, got[0], tc.value)
			})
		}
	})

	t.Run("should only answer without logger", func(t *testing.T) {
		m, err := createEnvelope(t, container.Bag{}, nil, nil)
		testutil.NoError(t, err)

		rec, panicked := serve(t, "/items/1", func(http.ResponseWriter, *http.Request) { panic(errBoom) }, m)
		testutil.Equal(t, "panicked", panicked, nil)
		testutil.Equal(t, "status", rec.Code, http.StatusInternalServerError)
		testutil.Equal(t, "body", rec.Body.String(), internalBody)
	})

	t.Run("should panic again with an abort handler panic, unreported", func(t *testing.T) {
		for name, value := range map[string]error{
			"abort":         http.ErrAbortHandler,
			"wrapped abort": fmt.Errorf("stop: %w", http.ErrAbortHandler),
		} {
			t.Run(name, func(t *testing.T) {
				logger := &entries{}
				m, err := createEnvelope(t, container.Bag{}, nil, logger)
				testutil.NoError(t, err)

				rec, panicked := serve(t, "/items/1", func(http.ResponseWriter, *http.Request) { panic(value) }, m)
				testutil.Equal(t, "panicked", panicked, any(value))
				testutil.Equal(t, "body", rec.Body.String(), "")
				testutil.Equal(t, "entries", len(logger.all()), 0)
			})
		}
	})

	t.Run("should report and panic again once the response was written", func(t *testing.T) {
		for name, value := range map[string]any{
			"error value":    errBoom,
			"envelope value": flamenvelope.NewEnvelope().WithNewError(http.StatusConflict, errBoom),
		} {
			t.Run(name, func(t *testing.T) {
				logger := &entries{}
				m, err := createEnvelope(t, container.Bag{}, nil, logger)
				testutil.NoError(t, err)

				rec, panicked := serve(t, "/items/1", func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte("partial"))
					panic(value)
				}, m)
				testutil.Equal(t, "panicked", panicked, value)
				testutil.Equal(t, "status", rec.Code, http.StatusCreated)
				testutil.Equal(t, "body", rec.Body.String(), "partial")
				got := logger.all()
				testutil.Equal(t, "count", len(got), 1)
				assertPanicEntry(t, got[0], value)
			})
		}
	})

	t.Run("should take the fields, then their defaults", func(t *testing.T) {
		logger := &entries{}
		m, err := createEnvelope(t, container.Bag{envelope.ConfigFieldChannel: "api"}, map[string]any{
			envelope.DefaultLevelConfigPath:        "warning",
			envelope.DefaultExposeErrorsConfigPath: true,
		}, logger)
		testutil.NoError(t, err)
		rec, _ := serve(t, "/items/1", func(http.ResponseWriter, *http.Request) { panic(errBoom) }, m)
		testutil.Equal(t, "exposed", strings.Contains(rec.Body.String(), `"detail":"boom"`), true)

		m, err = createEnvelope(t, container.Bag{envelope.ConfigFieldLevel: 4, envelope.ConfigFieldExposeErrors: false}, map[string]any{
			envelope.DefaultChannelConfigPath:      "web",
			envelope.DefaultExposeErrorsConfigPath: true,
		}, logger)
		testutil.NoError(t, err)
		rec, _ = serve(t, "/items/1", func(http.ResponseWriter, *http.Request) { panic(errBoom) }, m)
		testutil.Equal(t, "body", rec.Body.String(), internalBody)

		got := logger.all()
		testutil.Equal(t, "count", len(got), 2)
		testutil.Equal(t, "levels", []flam.LogLevel{got[0].level, got[1].level}, []flam.LogLevel{flam.LogWarning, flam.LogNotice})
		testutil.Equal(t, "channels", []string{got[0].channel, got[1].channel}, []string{"api", "web"})
	})

	t.Run("should use the built-in defaults when the defaults are missing", func(t *testing.T) {
		logger := &entries{}
		m, err := create(t, bare{envelope.NewBundle()}, envelope.Driver, container.Bag{}, nil, deps{logger: logger})
		testutil.NoError(t, err)
		rec, _ := serve(t, "/items/1", func(http.ResponseWriter, *http.Request) { panic(errBoom) }, m)
		testutil.Equal(t, "body", rec.Body.String(), internalBody)

		got := logger.all()
		testutil.Equal(t, "count", len(got), 1)
		testutil.Equal(t, "level", got[0].level, flam.LogError)
		testutil.Equal(t, "channel", got[0].channel, "flam.rest")
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, val := range map[string]any{
			envelope.ConfigFieldChannel:      "",
			envelope.ConfigFieldLevel:        "loud",
			envelope.ConfigFieldExposeErrors: "yes",
		} {
			_, err := createEnvelope(t, container.Bag{field: val}, nil, &entries{})
			testutil.Error(t, errors.Unwrap(err), envelope.ErrInvalidField,
				"unable to create envelope rest middleware: invalid envelope rest middleware field",
				map[string]any{envelope.ErrorFieldID: "main", envelope.ErrorFieldField: field})
		}
		for field, def := range map[string]string{
			envelope.ConfigFieldChannel:      envelope.DefaultChannelConfigPath,
			envelope.ConfigFieldExposeErrors: envelope.DefaultExposeErrorsConfigPath,
		} {
			_, err := createEnvelope(t, container.Bag{}, map[string]any{def: 42}, nil)
			testutil.Error(t, errors.Unwrap(err), envelope.ErrInvalidField,
				"unable to create envelope rest middleware: invalid envelope rest middleware field",
				map[string]any{envelope.ErrorFieldID: "main", envelope.ErrorFieldField: field})
		}
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := createEnvelope(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil, &entries{})
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
