package checks

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/afero"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/disk"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/memory"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	diskcheck "github.com/cjdias/flam-in-go/pkg/flam/health/checks/disk"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// diskApp returns an app with the disk factory, the memory disk "memory",
// the values, and the disk check driver.
func diskApp(t *testing.T, values map[string]any) *testutil.App {
	t.Helper()
	a := newApp(t, values, disk.NewBundle(), memory.NewBundle(), diskcheck.NewBundle())
	t.Cleanup(func() {
		_ = a.Invoke(func(f flam.IDiskFactory) error { return f.Close() })
	})
	return a
}

func Test_Disk_Declarations(t *testing.T) {
	testutil.Equal(t, "Driver", diskcheck.Driver, "flam.health.checks.driver.disk")
	testutil.Equal(t, "ConfigFieldDisk", diskcheck.ConfigFieldDisk, "disk")
	testutil.Equal(t, "ConfigFieldPath", diskcheck.ConfigFieldPath, "path")
	testutil.Equal(t, "ErrorFieldID", diskcheck.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldField", diskcheck.ErrorFieldField, "field")
	testutil.Equal(t, "ErrorFieldDisk", diskcheck.ErrorFieldDisk, "disk")
	testutil.Equal(t, "ErrorFieldPath", diskcheck.ErrorFieldPath, "path")
}

func Test_Disk_Bundle(t *testing.T) {
	b := diskcheck.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.health.checks.disk")

	a := newApp(t, nil, b)
	c := creatorOf(t, a)
	testutil.Equal(t, "Accept(driver)", c.Accept(checkBag("memory", diskcheck.Driver, nil)), true)
	testutil.Equal(t, "Accept(other)", c.Accept(checkBag("memory", "other", nil)), false)
}

func Test_Disk_Create(t *testing.T) {
	t.Run("should refuse an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			fields container.Bag
			field  string
		}{
			{container.Bag{health.ConfigFieldKind: 0}, health.ConfigFieldKind},
			{container.Bag{diskcheck.ConfigFieldPath: 1}, diskcheck.ConfigFieldPath},
			{container.Bag{diskcheck.ConfigFieldPath: ""}, diskcheck.ConfigFieldPath},
			{container.Bag{diskcheck.ConfigFieldDisk: 1}, diskcheck.ConfigFieldDisk},
			{container.Bag{diskcheck.ConfigFieldDisk: ""}, diskcheck.ConfigFieldDisk},
		} {
			a := diskApp(t, nil)
			check, err := creatorOf(t, a).Create(checkBag("memory", diskcheck.Driver, tc.fields))
			testutil.Error(t, err, diskcheck.ErrInvalidField,
				"unable to create disk health check: invalid disk health check field",
				map[string]any{diskcheck.ErrorFieldID: "memory", diskcheck.ErrorFieldField: tc.field})
			testutil.Equal(t, "check", check, nil)
		}
	})

	t.Run("should fail without disk factory", func(t *testing.T) {
		a := newApp(t, nil, diskcheck.NewBundle())
		_, err := creatorOf(t, a).Create(checkBag("memory", diskcheck.Driver, nil))
		testutil.Error(t, err, diskcheck.ErrNoFactory,
			"unable to create disk health check: no disk factory",
			map[string]any{diskcheck.ErrorFieldID: "memory"})
	})

	t.Run("should fail for an unknown disk", func(t *testing.T) {
		a := diskApp(t, nil)
		_, err := creatorOf(t, a).Create(checkBag("uploads", diskcheck.Driver, nil))
		testutil.Error(t, err, diskcheck.ErrUnknownDisk,
			"unable to create disk health check: unknown disk",
			map[string]any{diskcheck.ErrorFieldID: "uploads", diskcheck.ErrorFieldDisk: "uploads"})
	})

	t.Run("should check the readiness unless configured", func(t *testing.T) {
		a := diskApp(t, nil)
		c := creatorOf(t, a)

		check, err := c.Create(checkBag("memory", diskcheck.Driver, nil))
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", check.Id(), "memory")
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthReadiness)

		check, err = c.Create(checkBag("memory", diskcheck.Driver, container.Bag{health.ConfigFieldKind: flam.HealthBoth}))
		testutil.NoError(t, err)
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthBoth)
	})
}

func Test_Disk_Check(t *testing.T) {
	t.Run("should stat the root of the disk", func(t *testing.T) {
		a := diskApp(t, nil)
		check, err := creatorOf(t, a).Create(checkBag("memory", diskcheck.Driver, nil))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))
	})

	t.Run("should stat the path of the disk", func(t *testing.T) {
		a := diskApp(t, nil)
		testutil.NoError(t, a.Invoke(func(f flam.IDiskFactory) error {
			d, err := f.Get("memory")
			if err != nil {
				return err
			}
			return afero.WriteFile(d, "/reports/ready", []byte("ok"), 0o644)
		}))
		c := creatorOf(t, a)

		check, err := c.Create(checkBag("reports", diskcheck.Driver, container.Bag{
			diskcheck.ConfigFieldDisk: "memory",
			diskcheck.ConfigFieldPath: "/reports",
		}))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))

		check, err = c.Create(checkBag("reports", diskcheck.Driver, container.Bag{
			diskcheck.ConfigFieldDisk: "memory",
			diskcheck.ConfigFieldPath: "/missing",
		}))
		testutil.NoError(t, err)
		wantErr(t, check.Check(context.Background()),
			map[string]any{diskcheck.ErrorFieldID: "reports", diskcheck.ErrorFieldDisk: "memory", diskcheck.ErrorFieldPath: "/missing"},
			diskcheck.ErrStat, os.ErrNotExist)
	})

	t.Run("should fail when the disk cannot be created", func(t *testing.T) {
		a := diskApp(t, map[string]any{
			disk.DiskConfigPath + ".bad": container.Bag{factory.ConfigFieldDriver: "unknown"},
		})
		check, err := creatorOf(t, a).Create(checkBag("bad", diskcheck.Driver, nil))
		testutil.NoError(t, err)
		wantErr(t, check.Check(context.Background()),
			map[string]any{diskcheck.ErrorFieldID: "bad", diskcheck.ErrorFieldDisk: "bad"},
			diskcheck.ErrDisk, factory.ErrNoCreator)
	})
}
