package client

import (
	"errors"
	"testing"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/client"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var (
	errClient  = errors.New("client close")
	errBreaker = errors.New("breaker close")
)

// fakeClient is a client counting its closes, failing them with err.
type fakeClient struct {
	flam.IClient

	id     string
	err    error
	closes int
}

func (c *fakeClient) Id() string { return c.id }

func (c *fakeClient) Close() error {
	c.closes++
	return c.err
}

// fakeBreaker is a circuit breaker counting its closes, failing them with
// err.
type fakeBreaker struct {
	flam.IClientCircuitBreaker

	id     string
	err    error
	closes int
}

func (b *fakeBreaker) Id() string { return b.id }

func (b *fakeBreaker) Close() error {
	b.closes++
	return b.err
}

// clientCreator creates fake clients of the "fake" driver.
type clientCreator struct{}

func (clientCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "fake"
}

func (clientCreator) Create(bag flam.IBag) (flam.IClient, error) {
	return &fakeClient{id: bag.String(factory.ConfigFieldId)}, nil
}

// breakerCreator creates fake circuit breakers of the "fake" driver.
type breakerCreator struct{}

func (breakerCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "fake"
}

func (breakerCreator) Create(bag flam.IBag) (flam.IClientCircuitBreaker, error) {
	return &fakeBreaker{id: bag.String(factory.ConfigFieldId)}, nil
}

// newApp returns an app with the config and client bundles, the fake
// creators, and the clients and breakers of the ids configured with the
// fake driver.
func newApp(t *testing.T, clients []string, breakers []string) *testutil.App {
	t.Helper()
	a := testutil.NewApp()
	testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(client.NewBundle()))
	testutil.NoError(t, a.Provide(func() flam.IClientCreator { return clientCreator{} }, dig.Group(client.ClientCreatorGroup)))
	testutil.NoError(t, a.Provide(func() flam.IClientCircuitBreakerCreator { return breakerCreator{} }, dig.Group(client.BreakerCreatorGroup)))

	bag := container.Bag{}
	for _, id := range clients {
		testutil.NoError(t, bag.Set(client.ClientConfigPath+"."+id, container.Bag{factory.ConfigFieldDriver: "fake"}))
	}
	for _, id := range breakers {
		testutil.NoError(t, bag.Set(client.BreakerConfigPath+"."+id, container.Bag{factory.ConfigFieldDriver: "fake"}))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return a
}

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"ClientConfigPath":         {client.ClientConfigPath, "flam.client.clients"},
		"ClientCreatorGroup":       {client.ClientCreatorGroup, "flam.client.clients.creator"},
		"BreakerConfigPath":        {client.BreakerConfigPath, "flam.client.breakers"},
		"BreakerCreatorGroup":      {client.BreakerCreatorGroup, "flam.client.breakers.creator"},
		"DefaultBreakerConfigPath": {client.DefaultBreakerConfigPath, "flam.client.defaults.clients.breaker"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
}

func Test_ClientCircuitState(t *testing.T) {
	for state, want := range map[flam.ClientCircuitState]string{
		flam.ClientCircuitClosed:       "closed",
		flam.ClientCircuitOpen:         "open",
		flam.ClientCircuitHalfOpen:     "half-open",
		flam.ClientCircuitState(-1):    "unknown",
		flam.ClientCircuitHalfOpen + 1: "unknown",
	} {
		testutil.Equal(t, "String()", state.String(), want)
	}
	testutil.Equal(t, "states", []flam.ClientCircuitState{flam.ClientCircuitClosed, flam.ClientCircuitOpen, flam.ClientCircuitHalfOpen},
		[]flam.ClientCircuitState{0, 1, 2})
	testutil.Equal(t, "outcomes", []flam.ClientOutcome{flam.ClientOutcomeSuccess, flam.ClientOutcomeFailure, flam.ClientOutcomeIgnored},
		[]flam.ClientOutcome{0, 1, 2})
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the factories", func(t *testing.T) {
		b := client.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.client")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
		testutil.NoError(t, a.RegisterBundle(b))
		calls := a.Calls()
		testutil.Equal(t, "calls", calls[len(calls)-3:], []string{"bundle:flam.client", "provider/0", "provider/0"})
		testutil.NoError(t, a.Invoke(func(clients flam.IClientFactory, breakers flam.IClientCircuitBreakerFactory) {
			testutil.Equal(t, "clients", clients.Available(), []string{})
			testutil.Equal(t, "breakers", breakers.Available(), []string{})
		}))
	})

	t.Run("should create the configured resources with the creators", func(t *testing.T) {
		a := newApp(t, []string{"api", "web"}, []string{"main"})
		testutil.NoError(t, a.Invoke(func(clients flam.IClientFactory, breakers flam.IClientCircuitBreakerFactory) {
			testutil.Equal(t, "clients", clients.Available(), []string{"api", "web"})
			testutil.Equal(t, "breakers", breakers.Available(), []string{"main"})
			c, err := clients.Get("api")
			testutil.NoError(t, err)
			testutil.Equal(t, "client Id()", c.Id(), "api")
			b, err := breakers.Get("main")
			testutil.NoError(t, err)
			testutil.Equal(t, "breaker Id()", b.Id(), "main")
		}))
	})

	t.Run("should close the factories", func(t *testing.T) {
		a := newApp(t, []string{"api"}, []string{"main"})
		var c *fakeClient
		var b *fakeBreaker
		testutil.NoError(t, a.Invoke(func(clients flam.IClientFactory, breakers flam.IClientCircuitBreakerFactory) {
			got, _ := clients.Get("api")
			c = got.(*fakeClient)
			c.err = errClient
			gotBreaker, _ := breakers.Get("main")
			b = gotBreaker.(*fakeBreaker)
			b.err = errBreaker
		}))

		err := client.NewBundle().(flam.IClosableBundle).Close(a)
		if !errors.Is(err, errClient) || !errors.Is(err, errBreaker) {
			t.Fatalf("Close() = %v, want the errors of the client and of the breaker", err)
		}
		testutil.Equal(t, "client closes", c.closes, 1)
		testutil.Equal(t, "breaker closes", b.closes, 1)
		testutil.NoError(t, client.NewBundle().(flam.IClosableBundle).Close(a))
		testutil.Equal(t, "client closes again", c.closes, 1)
	})

	t.Run("should fail to close without the dependencies", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(client.NewBundle()))
		if err := client.NewBundle().(flam.IClosableBundle).Close(a); err == nil {
			t.Fatal("Close() did not fail")
		}
	})
}
