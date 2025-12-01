package tests

import (
	"errors"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_DirConfigSource(t *testing.T) {
	t.Run("should return dir opening error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverDir,
				"disk_id":   "my_disk",
				"path":      "/testdata",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk := afero.NewMemMapFs()

			require.NoError(t, factory.Store("my_disk", disk))
		}))

		assert.ErrorContains(t, app.Boot(), "file does not exist")
	})

	t.Run("should return dir reading error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverDir,
				"disk_id":   "my_disk",
				"path":      "/testdata",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("dir error")
		dirMock := mocks.NewMockFile(ctrl)
		dirMock.EXPECT().Readdir(0).Return(nil, expectedErr)
		dirMock.EXPECT().Close().Return(nil)

		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().Open("/testdata").Return(dirMock, nil)

		diskFactoryMock := mocks.NewMockDiskFactory(ctrl)
		diskFactoryMock.EXPECT().Get("my_disk").Return(diskMock, nil)
		diskFactoryMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Decorate(func(flam.DiskFactory) flam.DiskFactory {
			return diskFactoryMock
		}))

		assert.ErrorIs(t, app.Boot(), expectedErr)
	})

	t.Run("should correctly load an empty directory", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverDir,
				"disk_id":   "my_disk",
				"path":      "/testdata",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk := afero.NewMemMapFs()

			require.NoError(t, disk.MkdirAll("/testdata", os.ModePerm))

			require.NoError(t, factory.Store("my_disk", disk))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			bag := got.Get("", nil)
			assert.Empty(t, bag)
		}))
	})

	t.Run("should return the directory file opening error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverDir,
				"disk_id":   "my_disk",
				"path":      "/testdata",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		fileInfoMock := mocks.NewMockFileInfo(ctrl)
		fileInfoMock.EXPECT().IsDir().Return(false)
		fileInfoMock.EXPECT().Name().Return("file.yaml")

		dirMock := mocks.NewMockFile(ctrl)
		dirMock.EXPECT().Readdir(0).Return([]os.FileInfo{fileInfoMock}, nil)
		dirMock.EXPECT().Close().Return(nil)

		expectedErr := errors.New("file error")
		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().Open("/testdata").Return(dirMock, nil)
		diskMock.EXPECT().
			OpenFile("/testdata/file.yaml", os.O_RDONLY, os.FileMode(0o644)).
			Return(nil, expectedErr)

		diskFactoryMock := mocks.NewMockDiskFactory(ctrl)
		diskFactoryMock.EXPECT().Get("my_disk").Return(diskMock, nil)
		diskFactoryMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Decorate(func(flam.DiskFactory) flam.DiskFactory {
			return diskFactoryMock
		}))

		assert.ErrorIs(t, app.Boot(), expectedErr)
	})

	t.Run("should return the directory file parsing error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverDir,
				"disk_id":   "my_disk",
				"path":      "/testdata",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk := afero.NewMemMapFs()

			file, e := disk.Create("/testdata/file.yaml")
			require.NotNil(t, file)
			require.NoError(t, e)

			_, _ = file.WriteString("{")

			require.NoError(t, factory.Store("my_disk", disk))
		}))

		assert.ErrorContains(t, app.Boot(), "yaml: line 1: did not find expected node content")
	})

	t.Run("should correctly load directory files", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverDir,
				"disk_id":   "my_disk",
				"path":      "/testdata",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk := afero.NewMemMapFs()

			file, e := disk.Create("/testdata/file.yaml")
			require.NotNil(t, file)
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

	t.Run("should not load sub-directories if not flagged as recursive", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverDir,
				"disk_id":   "my_disk",
				"path":      "/testdata",
				"parser_id": "my_parser",
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk := afero.NewMemMapFs()

			file, e := disk.Create("/testdata/subdir/file.yaml")
			require.NotNil(t, file)
			require.NoError(t, e)

			_, _ = file.WriteString("field: value")

			require.NoError(t, factory.Store("my_disk", disk))
		}))

		assert.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.ConfigSourceFactory) {
			got, e := factory.Get("my_source")
			require.NotNil(t, got)
			require.NoError(t, e)

			assert.Nil(t, got.Get("field"))
		}))
	})

	t.Run("should return the sub-directory files opening error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverDir,
				"disk_id":   "my_disk",
				"path":      "/testdata",
				"parser_id": "my_parser",
				"recursive": true,
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		fileInfoMock := mocks.NewMockFileInfo(ctrl)
		fileInfoMock.EXPECT().IsDir().Return(false)
		fileInfoMock.EXPECT().Name().Return("file.yaml")

		subDirMock := mocks.NewMockFile(ctrl)
		subDirMock.EXPECT().Readdir(0).Return([]os.FileInfo{fileInfoMock}, nil)
		subDirMock.EXPECT().Close().Return(nil)

		subDirInfoMock := mocks.NewMockFileInfo(ctrl)
		subDirInfoMock.EXPECT().IsDir().Return(true)
		subDirInfoMock.EXPECT().Name().Return("subdir")

		dirMock := mocks.NewMockFile(ctrl)
		dirMock.EXPECT().Readdir(0).Return([]os.FileInfo{subDirInfoMock}, nil)
		dirMock.EXPECT().Close().Return(nil)

		expectedErr := errors.New("file error")
		diskMock := mocks.NewMockDisk(ctrl)
		diskMock.EXPECT().Open("/testdata").Return(dirMock, nil)
		diskMock.EXPECT().Open("/testdata/subdir").Return(subDirMock, nil)
		diskMock.EXPECT().
			OpenFile("/testdata/subdir/file.yaml", os.O_RDONLY, os.FileMode(0o644)).
			Return(nil, expectedErr)

		diskFactoryMock := mocks.NewMockDiskFactory(ctrl)
		diskFactoryMock.EXPECT().Get("my_disk").Return(diskMock, nil)
		diskFactoryMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Decorate(func(flam.DiskFactory) flam.DiskFactory {
			return diskFactoryMock
		}))

		assert.ErrorIs(t, app.Boot(), expectedErr)
	})

	t.Run("should return the sub-directory files parsing error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverDir,
				"disk_id":   "my_disk",
				"path":      "/testdata",
				"parser_id": "my_parser",
				"recursive": true,
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk := afero.NewMemMapFs()

			file, e := disk.Create("/testdata/subdir/file.yaml")
			require.NotNil(t, file)
			require.NoError(t, e)

			_, _ = file.WriteString("{")

			require.NoError(t, factory.Store("my_disk", disk))
		}))

		assert.ErrorContains(t, app.Boot(), "yaml: line 1: did not find expected node content")
	})

	t.Run("should correctly load sub-directory files if flagged as recursive", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathConfigBoot, true)
		_ = config.Set(flam.PathConfigParsers, flam.Bag{
			"my_parser": flam.Bag{
				"driver": flam.ConfigParserDriverYaml}})
		_ = config.Set(flam.PathConfigSources, flam.Bag{
			"my_source": flam.Bag{
				"driver":    flam.ConfigSourceDriverDir,
				"disk_id":   "my_disk",
				"path":      "/testdata",
				"parser_id": "my_parser",
				"recursive": true,
				"priority":  123}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Container().Invoke(func(factory flam.DiskFactory) {
			disk := afero.NewMemMapFs()

			file, e := disk.Create("/testdata/subdir/file.yaml")
			require.NotNil(t, file)
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
