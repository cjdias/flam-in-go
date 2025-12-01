package tests

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_Kennel_NewKennel(t *testing.T) {
	t.Run("should return process watchdog logger generation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_id": flam.Bag{
				"active":    true,
				"logger_id": "my_logger"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())
		assert.Error(t, app.Container().Invoke(func(_ flam.Kennel) {}))
	})

	t.Run("should initialize the kennel", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_id": flam.Bag{
				"active":    true,
				"logger_id": "my_logger"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id").AnyTimes()
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())
		assert.NoError(t, app.Container().Invoke(func(_ flam.Kennel) {}))
	})
}

func Test_Kennel_Available(t *testing.T) {
	t.Run("should list the available processes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id")
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(kennel flam.Kennel) {
			assert.ElementsMatch(t, []string{"process_id"}, kennel.Available())
		}))
	})
}

func Test_Kennel_Has(t *testing.T) {
	t.Run("should correctly return the process availability", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id")
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(kennel flam.Kennel) {
			assert.True(t, kennel.Has("process_id"))
			assert.False(t, kennel.Has("other_id"))
		}))
	})
}

func Test_Kennel_IsActive(t *testing.T) {
	t.Run("should correctly return the process activity", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_1_id": flam.Bag{
				"active": true},
			"process_2_id": flam.Bag{
				"active": false}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		processMock1 := mocks.NewMockProcess(ctrl)
		processMock1.EXPECT().Id().Return("process_1_id")
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock1
		}, dig.Group(flam.ProcessGroup)))

		processMock2 := mocks.NewMockProcess(ctrl)
		processMock2.EXPECT().Id().Return("process_2_id")
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock2
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(kennel flam.Kennel) {
			assert.True(t, kennel.IsActive("process_1_id"))
			assert.False(t, kennel.IsActive("process_2_id"))
			assert.False(t, kennel.IsActive("process_3_id"))
		}))
	})
}

func Test_Kennel_Activate(t *testing.T) {
	t.Run("should return error if the process was not found", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(kennel flam.Kennel) {
			assert.ErrorIs(t, kennel.Activate("process_id"), flam.ErrProcessNotFound)
		}))
	})

	t.Run("should return error on a running process", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_id": flam.Bag{
				"active": true}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id")
		processMock.EXPECT().IsRunning().Return(true)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(kennel flam.Kennel) {
			assert.ErrorIs(t, kennel.Activate("process_id"), flam.ErrProcessIsRunning)
		}))
	})

	t.Run("should correctly activate a non-running process", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_1_id": flam.Bag{
				"active": true},
			"process_2_id": flam.Bag{
				"active": false}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		processMock1 := mocks.NewMockProcess(ctrl)
		processMock1.EXPECT().Id().Return("process_1_id")
		processMock1.EXPECT().IsRunning().Return(false)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock1
		}, dig.Group(flam.ProcessGroup)))

		processMock2 := mocks.NewMockProcess(ctrl)
		processMock2.EXPECT().Id().Return("process_2_id")
		processMock2.EXPECT().IsRunning().Return(false)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock2
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(kennel flam.Kennel) {
			require.True(t, kennel.IsActive("process_1_id"))
			require.False(t, kennel.IsActive("process_2_id"))

			assert.NoError(t, kennel.Activate("process_1_id"))
			assert.NoError(t, kennel.Activate("process_2_id"))

			require.True(t, kennel.IsActive("process_1_id"))
			require.True(t, kennel.IsActive("process_2_id"))
		}))
	})
}

func Test_Kennel_Deactivate(t *testing.T) {
	t.Run("should return error if the process was not found", func(t *testing.T) {
		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(kennel flam.Kennel) {
			assert.ErrorIs(t, kennel.Deactivate("process_id"), flam.ErrProcessNotFound)
		}))
	})

	t.Run("should return error on a running process", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_id": flam.Bag{
				"active": true}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id")
		processMock.EXPECT().IsRunning().Return(true)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(kennel flam.Kennel) {
			assert.ErrorIs(t, kennel.Deactivate("process_id"), flam.ErrProcessIsRunning)
		}))
	})

	t.Run("should correctly deactivate a non-running process", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_1_id": flam.Bag{
				"active": true},
			"process_2_id": flam.Bag{
				"active": false}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		processMock1 := mocks.NewMockProcess(ctrl)
		processMock1.EXPECT().Id().Return("process_1_id")
		processMock1.EXPECT().IsRunning().Return(false)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock1
		}, dig.Group(flam.ProcessGroup)))

		processMock2 := mocks.NewMockProcess(ctrl)
		processMock2.EXPECT().Id().Return("process_2_id")
		processMock2.EXPECT().IsRunning().Return(false)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock2
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(kennel flam.Kennel) {
			require.True(t, kennel.IsActive("process_1_id"))
			require.False(t, kennel.IsActive("process_2_id"))

			assert.NoError(t, kennel.Deactivate("process_1_id"))
			assert.NoError(t, kennel.Deactivate("process_2_id"))

			require.False(t, kennel.IsActive("process_1_id"))
			require.False(t, kennel.IsActive("process_2_id"))
		}))
	})
}

func Test_Kennel_Run(t *testing.T) {
	t.Run("should not run processes if not flagged to do so", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication()
		defer func() { _ = app.Close() }()

		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id")
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Run())
	})

	t.Run("should not run process if not active", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathKennelRun, true)

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id")
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Run())
	})

	t.Run("should run process if active", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathKennelRun, true)
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_id": flam.Bag{
				"active": true}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id")
		processMock.EXPECT().Run().Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Run())
	})

	t.Run("should return process resulting error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathKennelRun, true)
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_id": flam.Bag{
				"active": true}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		expectedErr := errors.New("process error")
		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id")
		processMock.EXPECT().Run().Return(expectedErr)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.ErrorIs(t, app.Run(), expectedErr)
	})

	t.Run("should correctly log the process successful run", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathKennelRun, true)
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_id": flam.Bag{
				"active":    true,
				"logger_id": "my_logger"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "process [process_id] starting ...")
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "process [process_id] terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id").AnyTimes()
		processMock.EXPECT().Run().Return(nil)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Run())
	})

	t.Run("should recover a process panic error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathKennelRun, true)
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_id": flam.Bag{
				"active": true}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		run := 0
		expectedErr := errors.New("process error")
		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id")
		processMock.EXPECT().Run().DoAndReturn(func() error {
			run++
			if run == 1 {
				panic(expectedErr)
			}

			return nil
		}).Times(2)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Run())
	})

	t.Run("should correctly log the process recover", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathKennelRun, true)
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_id": flam.Bag{
				"active":    true,
				"logger_id": "my_logger"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "process [process_id] starting ...")
		loggerMock.EXPECT().Signal(flam.LogError, "flam", "process [process_id] error : process error")
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "process [process_id] terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		run := 0
		expectedErr := errors.New("process error")
		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id").AnyTimes()
		processMock.EXPECT().Run().DoAndReturn(func() error {
			run++
			if run == 1 {
				panic(expectedErr)
			}

			return nil
		}).Times(2)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Run())
	})

	t.Run("should correctly log the process recover (non-error panic)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathKennelRun, true)
		_ = config.Set(flam.PathWatchdogLoggers, flam.Bag{
			"my_logger": flam.Bag{
				"driver": flam.WatchdogLoggerDriverDefault}})
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_id": flam.Bag{
				"active":    true,
				"logger_id": "my_logger"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		loggerMock := mocks.NewMockLogger(ctrl)
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "process [process_id] starting ...")
		loggerMock.EXPECT().Signal(flam.LogError, "flam", "process [process_id] error : watchdog process running error: 123")
		loggerMock.EXPECT().Signal(flam.LogInfo, "flam", "process [process_id] terminated")
		require.NoError(t, app.Container().Decorate(func(flam.Logger) flam.Logger {
			return loggerMock
		}))

		run := 0
		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id").AnyTimes()
		processMock.EXPECT().Run().DoAndReturn(func() error {
			run++
			if run == 1 {
				panic(123)
			}

			return nil
		}).Times(2)
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Run())
	})
}

func Test_Kennel_Close(t *testing.T) {
	t.Run("should terminate any running the process", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathKennelRun, true)
		_ = config.Set(flam.PathProcesses, flam.Bag{
			"process_id": flam.Bag{
				"active": true}})

		app := flam.NewApplication(config)

		wg := sync.WaitGroup{}
		wg.Add(1)
		processMock := mocks.NewMockProcess(ctrl)
		processMock.EXPECT().Id().Return("process_id")
		processMock.EXPECT().Run().DoAndReturn(func() error {
			wg.Wait()
			return nil
		})
		processMock.EXPECT().Terminate().DoAndReturn(func() error {
			wg.Done()
			return nil
		})
		require.NoError(t, app.Container().Provide(func() flam.Process {
			return processMock
		}, dig.Group(flam.ProcessGroup)))

		require.NoError(t, app.Boot())

		timer := time.NewTimer(10 * time.Millisecond)
		go func() {
			<-timer.C
			assert.NoError(t, app.Container().Invoke(func(kennel flam.Kennel) {
				assert.NoError(t, kennel.Close())
			}))
		}()

		assert.NoError(t, app.Run())
	})
}
