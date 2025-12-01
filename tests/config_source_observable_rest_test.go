package tests

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_ObservableRestConfigSource(t *testing.T) {
	t.Run("should return requester error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("requester error")
		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(nil, expectedErr)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		assert.ErrorIs(t, app.Boot(), expectedErr)
	})

	t.Run("should return response read error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("requester error")
		bodyMock := mocks.NewMockReadCloser(ctrl)
		bodyMock.EXPECT().Read(gomock.Any()).Return(0, expectedErr)

		response := &http.Response{Body: bodyMock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		assert.ErrorIs(t, app.Boot(), expectedErr)
	})

	t.Run("should return response parsing error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := "{"
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		bodyMock := mocks.NewMockReadCloser(ctrl)
		bodyMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)

		response := &http.Response{Body: bodyMock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		assert.ErrorContains(t, app.Boot(), "unexpected end of JSON input")
	})

	t.Run("should return timestamp not found in response error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := "{}"
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		bodyMock := mocks.NewMockReadCloser(ctrl)
		bodyMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)

		response := &http.Response{Body: bodyMock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		assert.ErrorIs(t, app.Boot(), flam.ErrRestConfigSourceTimestampNotFound)
	})

	t.Run("should return invalid timestamp in response error (type)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := "{\"timestamp\": 123}"
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		bodyMock := mocks.NewMockReadCloser(ctrl)
		bodyMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)

		response := &http.Response{Body: bodyMock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidRestConfigSourceTimestamp)
	})

	t.Run("should return invalid timestamp in response error (string parsing)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := "{\"timestamp\": \"invalid\"}"
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		bodyMock := mocks.NewMockReadCloser(ctrl)
		bodyMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)

		response := &http.Response{Body: bodyMock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		assert.ErrorContains(t, app.Boot(), `cannot parse "invalid" as "2006"`)
	})

	t.Run("should return config not found in response error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})
		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := fmt.Sprintf(`{"timestamp": "%s"}`, time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		bodyMock := mocks.NewMockReadCloser(ctrl)
		bodyMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)

		response := &http.Response{Body: bodyMock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		assert.ErrorIs(t, app.Boot(), flam.ErrRestConfigSourceConfigNotFound)
	})

	t.Run("should return invalid config in response error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := fmt.Sprintf(`{"timestamp": "%s", "config": 123}`, time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		bodyMock := mocks.NewMockReadCloser(ctrl)
		bodyMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)

		response := &http.Response{Body: bodyMock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidRestConfigSourceConfig)
	})

	t.Run("should correctly load the config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := fmt.Sprintf(`{"timestamp": "%s", "config": {"field": "value"}}`, time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		bodyMock := mocks.NewMockReadCloser(ctrl)
		bodyMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)

		response := &http.Response{Body: bodyMock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		assert.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			assert.Equal(t, "value", got.Get("field"))
		}))
	})
}

func Test_ObservableRestConfigSource_Reload(t *testing.T) {
	t.Run("should return requester error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := fmt.Sprintf(`{"timestamp": "%s", "config": {"field": "value"}}`, time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		bodyMock := mocks.NewMockReadCloser(ctrl)
		bodyMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)

		response := &http.Response{Body: bodyMock}

		expectedErr := errors.New("requester error")
		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response, nil)
		requesterMock.EXPECT().Do(gomock.Any()).Return(nil, expectedErr)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		assert.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			assert.False(t, reloaded)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return response read error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"parser_id": "my_parser",
				"uri":       "http://path/",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := fmt.Sprintf(
			`{"timestamp": "%s", "config": {"field": "value"}}`,
			time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		body1Mock := mocks.NewMockReadCloser(ctrl)
		body1Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)

		expectedErr := errors.New("reader error")
		body2Mock := mocks.NewMockReadCloser(ctrl)
		body2Mock.EXPECT().Read(gomock.Any()).Return(0, expectedErr)

		response1 := &http.Response{Body: body1Mock}

		response2 := &http.Response{Body: body2Mock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response1, nil)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response2, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			assert.False(t, reloaded)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return response parsing error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data1 := fmt.Sprintf(
			`{"timestamp": "%s", "config": {"field": "value"}}`,
			time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader1 := func(b []byte) (int, error) {
			copy(b, data1)
			return len(data1), io.EOF
		}

		body1Mock := mocks.NewMockReadCloser(ctrl)
		body1Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader1)

		data2 := "{"
		reader2 := func(b []byte) (int, error) {
			copy(b, data2)
			return len(data2), io.EOF
		}

		body2Mock := mocks.NewMockReadCloser(ctrl)
		body2Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader2)

		response1 := &http.Response{Body: body1Mock}

		response2 := &http.Response{Body: body2Mock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response1, nil)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response2, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			assert.False(t, reloaded)
			assert.ErrorContains(t, e, "unexpected end of JSON input")
		}))
	})

	t.Run("should return timestamp not found in response error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data1 := fmt.Sprintf(
			`{"timestamp": "%s", "config": {"field": "value"}}`,
			time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader1 := func(b []byte) (int, error) {
			copy(b, data1)
			return len(data1), io.EOF
		}

		body1Mock := mocks.NewMockReadCloser(ctrl)
		body1Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader1)

		data2 := "{}"
		reader2 := func(b []byte) (int, error) {
			copy(b, data2)
			return len(data2), io.EOF
		}

		body2Mock := mocks.NewMockReadCloser(ctrl)
		body2Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader2)

		response1 := &http.Response{Body: body1Mock}

		response2 := &http.Response{Body: body2Mock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response1, nil)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response2, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			assert.False(t, reloaded)
			assert.ErrorIs(t, e, flam.ErrRestConfigSourceTimestampNotFound)
		}))
	})

	t.Run("should return invalid timestamp in response error (type)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data1 := fmt.Sprintf(
			`{"timestamp": "%s", "config": {"field": "value"}}`,
			time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader1 := func(b []byte) (int, error) {
			copy(b, data1)
			return len(data1), io.EOF
		}

		body1Mock := mocks.NewMockReadCloser(ctrl)
		body1Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader1)

		data2 := "{\"timestamp\": 1234567890}"
		reader2 := func(b []byte) (int, error) {
			copy(b, data2)
			return len(data2), io.EOF
		}

		body2Mock := mocks.NewMockReadCloser(ctrl)
		body2Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader2)

		response1 := &http.Response{Body: body1Mock}

		response2 := &http.Response{Body: body2Mock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response1, nil)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response2, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			assert.False(t, reloaded)
			assert.ErrorIs(t, e, flam.ErrInvalidRestConfigSourceTimestamp)
		}))
	})

	t.Run("should return invalid timestamp in response error (string parsing)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data1 := fmt.Sprintf(
			`{"timestamp": "%s", "config": {"field": "value"}}`,
			time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader1 := func(b []byte) (int, error) {
			copy(b, data1)
			return len(data1), io.EOF
		}

		body1Mock := mocks.NewMockReadCloser(ctrl)
		body1Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader1)

		data2 := "{\"timestamp\": \"invalid\"}"
		reader2 := func(b []byte) (int, error) {
			copy(b, data2)
			return len(data2), io.EOF
		}

		body2Mock := mocks.NewMockReadCloser(ctrl)
		body2Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader2)

		response1 := &http.Response{Body: body1Mock}

		response2 := &http.Response{Body: body2Mock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response1, nil)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response2, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			assert.False(t, reloaded)
			assert.ErrorContains(t, e, `cannot parse "invalid" as "2006"`)
		}))
	})

	t.Run("should return config not found in response error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data1 := fmt.Sprintf(
			`{"timestamp": "%s", "config": {"field": "value"}}`,
			time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader1 := func(b []byte) (int, error) {
			copy(b, data1)
			return len(data1), io.EOF
		}

		body1Mock := mocks.NewMockReadCloser(ctrl)
		body1Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader1)

		data2 := fmt.Sprintf(`{"timestamp": "%s"}`, time.Now().Add(time.Hour*25).Format(time.RFC3339))
		reader2 := func(b []byte) (int, error) {
			copy(b, data2)
			return len(data2), io.EOF
		}

		body2Mock := mocks.NewMockReadCloser(ctrl)
		body2Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader2)

		response1 := &http.Response{Body: body1Mock}

		response2 := &http.Response{Body: body2Mock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response1, nil)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response2, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			assert.False(t, reloaded)
			assert.ErrorIs(t, e, flam.ErrRestConfigSourceConfigNotFound)
		}))
	})

	t.Run("should return invalid config in response error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data1 := fmt.Sprintf(
			`{"timestamp": "%s", "config": {"field": "value"}}`,
			time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader1 := func(b []byte) (int, error) {
			copy(b, data1)
			return len(data1), io.EOF
		}

		body1Mock := mocks.NewMockReadCloser(ctrl)
		body1Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader1)

		data2 := fmt.Sprintf(
			`{"timestamp": "%s", "config": 123}`,
			time.Now().Add(time.Hour*25).Format(time.RFC3339))
		reader2 := func(b []byte) (int, error) {
			copy(b, data2)
			return len(data2), io.EOF
		}

		body2Mock := mocks.NewMockReadCloser(ctrl)
		body2Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader2)

		response1 := &http.Response{Body: body1Mock}

		response2 := &http.Response{Body: body2Mock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response1, nil)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response2, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			assert.False(t, reloaded)
			assert.ErrorIs(t, e, flam.ErrInvalidRestConfigSourceConfig)
		}))
	})

	t.Run("should correctly reload the source", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data1 := fmt.Sprintf(
			`{"timestamp": "%s", "config": {"field": "value"}}`,
			time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader1 := func(b []byte) (int, error) {
			copy(b, data1)
			return len(data1), io.EOF
		}

		body1Mock := mocks.NewMockReadCloser(ctrl)
		body1Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader1)

		data2 := fmt.Sprintf(`{"timestamp": "%s", "config": {"field": "value2"}}`, time.Now().Add(time.Hour*25).Format(time.RFC3339))
		reader2 := func(b []byte) (int, error) {
			copy(b, data2)
			return len(data2), io.EOF
		}

		body2Mock := mocks.NewMockReadCloser(ctrl)
		body2Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader2)

		response1 := &http.Response{Body: body1Mock}

		response2 := &http.Response{Body: body2Mock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response1, nil)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response2, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			assert.True(t, reloaded)
			assert.NoError(t, e)

			assert.Equal(t, "value2", got.Get("field"))
		}))
	})

	t.Run("should not reload config if the timestamp is less or equals to the stored timestamp", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})
		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data1 := fmt.Sprintf(`{"timestamp": "%s", "config": {"field": "value"}}`, time.Now().Add(time.Hour*24).Format(time.RFC3339))
		reader1 := func(b []byte) (int, error) {
			copy(b, data1)
			return len(data1), io.EOF
		}

		body1Mock := mocks.NewMockReadCloser(ctrl)
		body1Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader1)

		data2 := fmt.Sprintf(`{"timestamp": "%s", "config": {"field": "value2"}}`, time.Now().Add(time.Hour*23).Format(time.RFC3339))
		reader2 := func(b []byte) (int, error) {
			copy(b, data2)
			return len(data2), io.EOF
		}

		body2Mock := mocks.NewMockReadCloser(ctrl)
		body2Mock.EXPECT().Read(gomock.Any()).DoAndReturn(reader2)

		response1 := &http.Response{Body: body1Mock}

		response2 := &http.Response{Body: body2Mock}

		requesterMock := mocks.NewMockConfigRestClient(ctrl)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response1, nil)
		requesterMock.EXPECT().Do(gomock.Any()).Return(response2, nil)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			assert.False(t, reloaded)
			assert.NoError(t, e)

			assert.Equal(t, "value", got.Get("field"))
		}))
	})
}
