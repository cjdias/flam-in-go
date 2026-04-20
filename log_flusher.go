package flam

import "time"

type logFlusher struct {
	config         Config
	logger         Logger
	triggerFactory TriggerFactory
	trigger        Trigger
}

func newLogFlusher(
	config Config,
	logger Logger,
	triggerFactory TriggerFactory,
) *logFlusher {
	return &logFlusher{
		config:         config,
		logger:         logger,
		triggerFactory: triggerFactory,
		trigger:        nil}
}

func (flusher *logFlusher) Close() error {
	if flusher.trigger != nil {
		return flusher.trigger.Close()
	}

	return nil
}

func (flusher *logFlusher) Boot() error {
	if !flusher.config.Bool(PathLogBoot) {
		return nil
	}

	frequency := flusher.config.Duration(PathLogFlusherFrequency)
	if frequency != time.Duration(0) {
		trigger, e := flusher.triggerFactory.NewRecurring(frequency, flusher.Callback)
		if e != nil {
			return e
		}
		flusher.trigger = trigger
	}

	if e := flusher.config.AddObserver(
		"flam.log",
		PathLogFlusherFrequency,
		func(old, new any) {
			newFrequency, ok := new.(time.Duration)
			if !ok {
				return
			}

			if flusher.trigger != nil {
				// Error ignored - trigger close is cleanup, shouldn't block observer update
				_ = flusher.trigger.Close()
			}

			// Error ignored - if trigger creation fails, flusher will continue without auto-flush
			flusher.trigger, _ = flusher.triggerFactory.NewRecurring(newFrequency, flusher.Callback)
		},
	); e != nil {
		return e
	}

	return nil
}

func (flusher *logFlusher) Callback() error {
	return flusher.logger.Flush()
}
