package cache_test

import (
	"context"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators/field"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/serializers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	flamredis "github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
)

// user is the cached object, keyed by its Id.
type user struct {
	Id   uint
	Name string
}

func ExampleNewBundle() {
	// a redis adaptor over an in-memory redis server of the mini driver
	a := app.New(container.Bag{"flam": container.Bag{
		"redis": container.Bag{"connections": container.Bag{
			"main": container.Bag{"driver": mini.Driver},
		}},
		"cache": container.Bag{"adaptors": container.Bag{
			"main": container.Bag{"driver": redis.Driver, "connection": "main"},
		}},
	}})
	defer func() { _ = a.Close() }()

	for _, b := range []flam.IBundle{
		flamredis.NewBundle(), mini.NewBundle(),
		cache.NewBundle(), json.NewBundle(), field.NewBundle(), redis.NewBundle(),
	} {
		if err := a.RegisterBundle(b); err != nil {
			fmt.Println(err)
			return
		}
	}

	err := a.Invoke(func(f flam.ICacheAdaptorFactory) error {
		adaptor, err := f.Get("main")
		if err != nil {
			return err
		}
		ctx := context.Background()
		if err := adaptor.Set(ctx, &user{Id: 1, Name: "ann"}); err != nil {
			return err
		}

		// objects holding only their identity fetch the cached ones
		ann, bob := &user{Id: 1}, &user{Id: 2}
		found, err := adaptor.Get(ctx, ann, bob)
		if err != nil {
			return err
		}
		fmt.Println(ann.Name, found[0] != nil, found[1] != nil)
		return nil
	})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// ann true false
}
