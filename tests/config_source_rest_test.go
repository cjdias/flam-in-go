package tests

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_RestConfigSource(t *testing.T) {
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
				"driver":    flam.ConfigSourceDriverRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config": "config"},
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
				"driver":    flam.ConfigSourceDriverRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config": "config"},
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
				"driver":    flam.ConfigSourceDriverRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config": "config"},
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
				"driver":    flam.ConfigSourceDriverRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config": "config"},
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
				"driver":    flam.ConfigSourceDriverRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config": "config"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := `{"config": "invalid"}`
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
				"driver":    flam.ConfigSourceDriverRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config": "config"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := `{"config": {"field": "value"}}`
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
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.Equal(t, "value", got.Get("field"))
		}))
	})
}
