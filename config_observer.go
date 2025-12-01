package flam

import "time"

type configObserver struct {
	config              Config
	configSourceFactory ConfigSourceFactory
	triggerFactory      TriggerFactory
	trigger             Trigger
}

func newConfigObserver(
	config Config,
	configSourceFactory ConfigSourceFactory,
	triggerFactory TriggerFactory,
) *configObserver {
	return &configObserver{
		config:              config,
		configSourceFactory: configSourceFactory,
		triggerFactory:      triggerFactory}
}

func (observer *configObserver) Close() error {
	if observer.trigger != nil {
		return observer.trigger.Close()
	}

	return nil
}

func (observer *configObserver) Boot() error {
	if !observer.config.Bool(PathConfigBoot) {
		return nil
	}

	frequency := observer.config.Duration(PathConfigObserverFrequency)
	if frequency != time.Duration(0) {
		trigger, e := observer.triggerFactory.NewRecurring(frequency, observer.Callback)
		if e != nil {
			return e
		}
		observer.trigger = trigger
	}

	if e := observer.config.AddObserver(
		"flam.config",
		PathConfigObserverFrequency,
		func(old, new any) {
			newFrequency, ok := new.(time.Duration)
			if !ok {
				return
			}

			if observer.trigger != nil {
				_ = observer.trigger.Close()
			}

			observer.trigger, _ = observer.triggerFactory.NewRecurring(newFrequency, observer.Callback)
		},
	); e != nil {
		return e
	}

	return nil
}

func (observer *configObserver) Callback() error {
	return observer.configSourceFactory.Reload()
}
