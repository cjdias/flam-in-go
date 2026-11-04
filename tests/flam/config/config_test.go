package config

import (
	"errors"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam/config"
)

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	sentinels := []error{
		config.ErrInvalidTarget, config.ErrMissingPath, config.ErrNilObserver,
		config.ErrDuplicateObserver, config.ErrUnknownObserver,
		config.ErrNilSource, config.ErrDuplicateSource, config.ErrUnknownSource,
		config.ErrMissingTriggerFactory,
	}
	for i, err := range sentinels {
		for j, other := range sentinels {
			if i != j && errors.Is(err, other) {
				t.Errorf("%q matches %q", err, other)
			}
		}
	}
}

func Test_Sentinels(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		text string
	}{
		{"ErrInvalidTarget", config.ErrInvalidTarget, "invalid config populate target"},
		{"ErrMissingPath", config.ErrMissingPath, "config path not found"},
		{"ErrNilObserver", config.ErrNilObserver, "nil config observer"},
		{"ErrDuplicateObserver", config.ErrDuplicateObserver, "config observer id already in use"},
		{"ErrUnknownObserver", config.ErrUnknownObserver, "unknown config observer id"},
		{"ErrNilSource", config.ErrNilSource, "nil config source"},
		{"ErrDuplicateSource", config.ErrDuplicateSource, "config source id already in use"},
		{"ErrUnknownSource", config.ErrUnknownSource, "unknown config source id"},
		{"ErrMissingTriggerFactory", config.ErrMissingTriggerFactory, "trigger factory not provided by the container"},
	} {
		t.Run("should declare "+tc.name+" with its message", func(t *testing.T) {
			if tc.err == nil {
				t.Fatalf("%s is nil", tc.name)
			}
			if tc.err.Error() != tc.text {
				t.Errorf("%s.Error() = %q, want %q", tc.name, tc.err.Error(), tc.text)
			}
			if errors.Unwrap(tc.err) != nil {
				t.Errorf("%s wraps %v", tc.name, errors.Unwrap(tc.err))
			}
		})
	}
}

func Test_Constants(t *testing.T) {
	for _, tc := range []struct {
		name, got, want string
	}{
		{"ParserConfigPath", config.ParserConfigPath, "flam.config.parsers"},
		{"ParserCreatorGroup", config.ParserCreatorGroup, "flam.config.parsers.creator"},
		{"SourceConfigPath", config.SourceConfigPath, "flam.config.sources"},
		{"SourceCreatorGroup", config.SourceCreatorGroup, "flam.config.sources.creator"},
		{"BootConfigPath", config.BootConfigPath, "flam.config.boot"},
		{"ReloadPeriodConfigPath", config.ReloadPeriodConfigPath, "flam.config.reloader.period"},
		{"ReloaderTriggerID", config.ReloaderTriggerID, "flam.config.reloader"},
	} {
		t.Run("should declare "+tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}
