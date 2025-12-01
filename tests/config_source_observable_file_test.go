package tests

import (
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_ObservableFileConfigSource(t *testing.T) {
	t.Run("should return file stat error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("file error")
		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().Stat("/testdata/config").Return(nil, expectedErr)

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk", diskMock))
		}))

		assert.ErrorIs(t, app.Boot(), expectedErr)
	})

	t.Run("should return file opening error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		now := time.Now()
		fileInfoMock := mocks.NewMockFileInfo(ctrl)
		fileInfoMock.EXPECT().ModTime().Return(now)

		expectedErr := errors.New("file error")
		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().Stat("/testdata/config").Return(fileInfoMock, nil)
		diskMock.EXPECT().
			OpenFile("/testdata/config", os.O_RDONLY, os.FileMode(0o644)).
			Return(nil, expectedErr)

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk", diskMock))
		}))

		assert.ErrorIs(t, app.Boot(), expectedErr)
	})

	t.Run("should return file parsing error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk := afero.NewMemMapFs()

			file, e := disk.Create("/testdata/config")
			require.NoError(t, e)
			require.NoError(t, e)

			_, _ = file.WriteString("{")

			require.NoError(t, factory.Store("my_disk", disk))
		}))

		assert.ErrorContains(t, app.Boot(), "yaml: line 1: did not find expected node content")
	})

	t.Run("should correctly load observable file source", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk := afero.NewMemMapFs()

			file, e := disk.Create("/testdata/config")
			require.NoError(t, e)
			require.NoError(t, e)

			_, _ = file.WriteString("field: value")

			require.NoError(t, factory.Store("my_disk", disk))
		}))

		assert.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.Equal(t, "value", got.Get("field"))
		}))
	})
}

func Test_ObservableFileConfigSource_Reload(t *testing.T) {
	t.Run("should return file stat error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := "field: value"
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		fileMock := mocks.NewMockFile(ctrl)
		fileMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)
		fileMock.EXPECT().Close().Return(nil)

		now := time.Now()
		fileInfoMock := mocks.NewMockFileInfo(ctrl)
		fileInfoMock.EXPECT().ModTime().Return(now)

		expectedErr := errors.New("filesystem error")
		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().Stat("/testdata/config").Return(fileInfoMock, nil)
		diskMock.EXPECT().Stat("/testdata/config").Return(nil, expectedErr)
		diskMock.EXPECT().
			OpenFile("/testdata/config", os.O_RDONLY, os.FileMode(0o644)).
			Return(fileMock, nil)

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk", diskMock))
		}))

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

	t.Run("should no-op if the file was not updated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := "field: value"
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		fileMock := mocks.NewMockFile(ctrl)
		fileMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)
		fileMock.EXPECT().Close().Return(nil)

		now := time.Now()
		fileInfoMock := mocks.NewMockFileInfo(ctrl)
		fileInfoMock.EXPECT().ModTime().Return(now).Times(2)

		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().Stat("/testdata/config").Return(fileInfoMock, nil).Times(2)
		diskMock.EXPECT().
			OpenFile("/testdata/config", os.O_RDONLY, os.FileMode(0o644)).
			Return(fileMock, nil)

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk", diskMock))
		}))

		assert.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			assert.False(t, reloaded)
			assert.NoError(t, e)
		}))
	})

	t.Run("should return file opening error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := "field: value"
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		fileMock := mocks.NewMockFile(ctrl)
		fileMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)
		fileMock.EXPECT().Close().Return(nil)

		now := time.Now()
		future := now.AddDate(1, 0, 0)
		fileInfoMock := mocks.NewMockFileInfo(ctrl)
		fileInfoMock.EXPECT().ModTime().Return(now)
		fileInfoMock.EXPECT().ModTime().Return(future)

		expectedErr := errors.New("filesystem error")
		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().Stat("/testdata/config").Return(fileInfoMock, nil).Times(2)
		diskMock.EXPECT().
			OpenFile("/testdata/config", os.O_RDONLY, os.FileMode(0o644)).
			Return(fileMock, nil)
		diskMock.EXPECT().
			OpenFile("/testdata/config", os.O_RDONLY, os.FileMode(0o644)).
			Return(nil, expectedErr)

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk", diskMock))
		}))

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

	t.Run("should return file reading error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data := "field: value"
		reader := func(b []byte) (int, error) {
			copy(b, data)
			return len(data), io.EOF
		}

		expectedErr := errors.New("filesystem error")
		fileMock := mocks.NewMockFile(ctrl)
		fileMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader)
		fileMock.EXPECT().Read(gomock.Any()).Return(0, expectedErr)
		fileMock.EXPECT().Close().Return(nil).Times(2)

		now := time.Now()
		future := now.AddDate(1, 0, 0)
		fileInfoMock := mocks.NewMockFileInfo(ctrl)
		fileInfoMock.EXPECT().ModTime().Return(now)
		fileInfoMock.EXPECT().ModTime().Return(future)

		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().Stat("/testdata/config").Return(fileInfoMock, nil).Times(2)
		diskMock.EXPECT().
			OpenFile("/testdata/config", os.O_RDONLY, os.FileMode(0o644)).
			Return(fileMock, nil).
			Times(2)

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk", diskMock))
		}))

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

	t.Run("should return file parsing error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data1 := "field: value"
		reader1 := func(b []byte) (int, error) {
			copy(b, data1)
			return len(data1), io.EOF
		}

		data2 := "{"
		reader2 := func(b []byte) (int, error) {
			copy(b, data2)
			return len(data2), io.EOF
		}

		fileMock := mocks.NewMockFile(ctrl)
		fileMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader1)
		fileMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader2)
		fileMock.EXPECT().Close().Return(nil).Times(2)

		now := time.Now()
		future := now.AddDate(1, 0, 0)
		fileInfoMock := mocks.NewMockFileInfo(ctrl)
		fileInfoMock.EXPECT().ModTime().Return(now)
		fileInfoMock.EXPECT().ModTime().Return(future)

		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().Stat("/testdata/config").Return(fileInfoMock, nil).Times(2)
		diskMock.EXPECT().
			OpenFile("/testdata/config", os.O_RDONLY, os.FileMode(0o644)).
			Return(fileMock, nil).
			Times(2)

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk", diskMock))
		}))

		assert.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			assert.False(t, reloaded)
			assert.ErrorContains(t, e, "yaml: line 1: did not find expected node content")
		}))
	})

	t.Run("should correctly reload the source", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverObservableFile,
				"disk_id":   "my_disk",
				"path":      "/testdata/config",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		data1 := "field: value"
		reader1 := func(b []byte) (int, error) {
			copy(b, data1)
			return len(data1), io.EOF
		}

		data2 := "field2: value2"
		reader2 := func(b []byte) (int, error) {
			copy(b, data2)
			return len(data2), io.EOF
		}

		fileMock := mocks.NewMockFile(ctrl)
		fileMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader1)
		fileMock.EXPECT().Read(gomock.Any()).DoAndReturn(reader2)
		fileMock.EXPECT().Close().Return(nil).Times(2)

		now := time.Now()
		future := now.AddDate(1, 0, 0)
		fileInfoMock := mocks.NewMockFileInfo(ctrl)
		fileInfoMock.EXPECT().ModTime().Return(now)
		fileInfoMock.EXPECT().ModTime().Return(future)

		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().Stat("/testdata/config").Return(fileInfoMock, nil).Times(2)
		diskMock.EXPECT().
			OpenFile("/testdata/config", os.O_RDONLY, os.FileMode(0o644)).
			Return(fileMock, nil).
			Times(2)

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			require.NoError(t, factory.Store("my_disk", diskMock))
		}))

		assert.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			reloaded, e := got.(flam.ObservableConfigSource).Reload()
			require.True(t, reloaded)
			require.NoError(t, e)

			assert.Nil(t, got.Get("field"))
			assert.Equal(t, "value2", got.Get("field2"))
		}))
	})
}
