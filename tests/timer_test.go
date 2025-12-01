package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/cjdias/flam-in-go"
)

func Test_Timer_ParseDuration(t *testing.T) {
	app := flam.NewApplication()
	defer func() { _ = app.Close() }()

	assert.NoError(t, app.Container().Invoke(func(timer flam.Timer) {
		got, e := timer.ParseDuration("1s")
		assert.Equal(t, time.Second, got)
		assert.NoError(t, e)
	}))
}

func Test_Timer_Now(t *testing.T) {
	app := flam.NewApplication()
	defer func() { _ = app.Close() }()

	assert.NoError(t, app.Container().Invoke(func(timer flam.Timer) {
		assert.WithinDuration(t, time.Now(), timer.Now(), 10*time.Millisecond)
	}))
}

func Test_Timer_Since(t *testing.T) {
	app := flam.NewApplication()
	defer func() { _ = app.Close() }()

	assert.NoError(t, app.Container().Invoke(func(timer flam.Timer) {
		then := time.Now().Add(-5 * time.Second)
		assert.GreaterOrEqual(t, timer.Since(then), 5*time.Second)
	}))
}

func Test_Timer_Until(t *testing.T) {
	app := flam.NewApplication()
	defer func() { _ = app.Close() }()

	assert.NoError(t, app.Container().Invoke(func(timer flam.Timer) {
		future := time.Now().Add(5 * time.Second)
		assert.GreaterOrEqual(t, timer.Until(future), 4*time.Second)
		assert.LessOrEqual(t, timer.Until(future), 5*time.Second)
	}))
}

func Test_Timer_FixedZone(t *testing.T) {
	app := flam.NewApplication()
	defer func() { _ = app.Close() }()

	assert.NoError(t, app.Container().Invoke(func(timer flam.Timer) {
		got := timer.FixedZone("test", 3600)
		assert.NotNil(t, got)
		assert.Equal(t, "test", got.String())
	}))
}

func Test_Timer_LoadLocation(t *testing.T) {
	app := flam.NewApplication()
	defer func() { _ = app.Close() }()

	assert.NoError(t, app.Container().Invoke(func(timer flam.Timer) {
		got, e := timer.LoadLocation("UTC")
		assert.NotNil(t, got)
		assert.NoError(t, e)

		assert.Equal(t, "UTC", got.String())
	}))
}

func Test_Timer_Date(t *testing.T) {
	app := flam.NewApplication()
	defer func() { _ = app.Close() }()

	assert.NoError(t, app.Container().Invoke(func(timer flam.Timer) {
		loc, _ := time.LoadLocation("UTC")
		got := timer.Date(2024, time.January, 1, 10, 30, 0, 0, loc)
		assert.Equal(t, 2024, got.Year())
		assert.Equal(t, time.January, got.Month())
	}))
}

func Test_Timer_Parse(t *testing.T) {
	app := flam.NewApplication()
	defer func() { _ = app.Close() }()

	assert.NoError(t, app.Container().Invoke(func(timer flam.Timer) {
		got, e := timer.Parse(time.RFC3339, "2024-01-01T10:30:00Z")
		assert.Equal(t, 2024, got.Year())
		assert.NoError(t, e)
	}))
}

func Test_Timer_ParseInLocation(t *testing.T) {
	app := flam.NewApplication()
	defer func() { _ = app.Close() }()

	assert.NoError(t, app.Container().Invoke(func(timer flam.Timer) {
		loc, _ := time.LoadLocation("UTC")
		got, e := timer.ParseInLocation(time.RFC3339, "2024-01-01T10:30:00Z", loc)
		assert.Equal(t, 2024, got.Year())
		assert.NoError(t, e)
	}))
}

func Test_Timer_Unix(t *testing.T) {
	app := flam.NewApplication()
	defer func() { _ = app.Close() }()

	assert.NoError(t, app.Container().Invoke(func(timer flam.Timer) {
		ts := time.Now().Unix()
		assert.Equal(t, ts, timer.Unix(ts, 0).Unix())
	}))
}

func Test_Timer_UnixMicro(t *testing.T) {
	app := flam.NewApplication()
	defer func() { _ = app.Close() }()

	assert.NoError(t, app.Container().Invoke(func(timer flam.Timer) {
		ts := time.Now().UnixMicro()
		assert.Equal(t, ts, timer.UnixMicro(ts).UnixMicro())
	}))
}

func Test_Timer_UnixMilli(t *testing.T) {
	app := flam.NewApplication()
	defer func() { _ = app.Close() }()

	assert.NoError(t, app.Container().Invoke(func(timer flam.Timer) {
		ts := time.Now().UnixMilli()
		assert.Equal(t, ts, timer.UnixMilli(ts).UnixMilli())
	}))
}
