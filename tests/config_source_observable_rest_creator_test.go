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

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_ObservableRestConfigSourceCreator(t *testing.T) {
	t.Run("should ignore config without/empty uri field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidResourceConfig)
	})

	t.Run("should ignore config without/empty parser_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidResourceConfig)
	})

	t.Run("should ignore config without/empty path.config field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "",
					"timestamp": "timestamp"},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidResourceConfig)
	})

	t.Run("should ignore config without/empty path.timestamp field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": ""},
				"priority": 123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		assert.ErrorIs(t, app.Boot(), flam.ErrInvalidResourceConfig)
	})

	t.Run("should return requester generation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
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
		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(nil, expectedErr)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		assert.ErrorIs(t, app.Boot(), expectedErr)
	})

	t.Run("should return parser generation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
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

		requesterMock := mocks.NewMockConfigRestClient(ctrl)

		requesterGeneratorMock := mocks.NewMockConfigRestClientGenerator(ctrl)
		requesterGeneratorMock.EXPECT().Create().Return(requesterMock, nil)
		_ = app.Container().Decorate(func(flam.ConfigRestClientGenerator) flam.ConfigRestClientGenerator {
			return requesterGeneratorMock
		})

		assert.ErrorIs(t, app.Boot(), flam.ErrUnknownResource)
	})

	t.Run("should generate with default priority if not given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigDefaultPriority, 123)
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
					"timestamp": "timestamp"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := fmt.Sprintf(
			`{"timestamp": "%s", "config": {"field": "value"}}`,
			time.Now().Add(time.Hour*24).Format(time.RFC3339))
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

			assert.Equal(t, 123, got.GetPriority())
		}))
	})

	t.Run("should generate with default parser if not given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigDefaultRestParserId, "my_parser")
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver": flam.ConfigSourceDriverObservableRest,
				"uri":    "http://path/",
				"path": flam.Bag{
					"config":    "config",
					"timestamp": "timestamp"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := fmt.Sprintf(
			`{"timestamp": "%s", "config": {"field": "value"}}`,
			time.Now().Add(time.Hour*24).Format(time.RFC3339))
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
	})

	t.Run("should generate with default config path if not given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigDefaultRestConfigPath, "config")
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"timestamp": "timestamp"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := fmt.Sprintf(
			`{"timestamp": "%s", "config": {"field": "value"}}`,
			time.Now().Add(time.Hour*24).Format(time.RFC3339))
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
	})

	t.Run("should generate with default timestamp path if not given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigDefaultRestTimestampPath, "timestamp")
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverJson}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableRest,
				"uri":       "http://path/",
				"parser_id": "my_parser",
				"path": flam.Bag{
					"config": "config"}}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := fmt.Sprintf(
			`{"timestamp": "%s", "config": {"field": "value"}}`,
			time.Now().Add(time.Hour*24).Format(time.RFC3339))
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
	})
}
