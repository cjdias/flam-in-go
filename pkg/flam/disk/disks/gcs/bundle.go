package gcs

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/disk"
)

// NewBundle returns the [flam.IBundle] of the gcs disk driver, providing
// its [flam.IDiskCreator] in the [disk.DiskCreatorGroup] group. It is also
// a [flam.IConfigurableBundle], setting the defaults of the disk fields
// at [DefaultEndpointConfigPath], [DefaultCredentialsTypeConfigPath] and
// [DefaultAnonymousConfigPath]. It configures no disk, as a disk needs a
// bucket.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IConfigurableBundle
var _ flam.IConfigurableBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Config(
	bag flam.IBag,
) error {
	for path, value := range map[string]any{
		DefaultEndpointConfigPath:        defaultEndpoint,
		DefaultCredentialsTypeConfigPath: defaultCredentialsType,
		DefaultAnonymousConfigPath:       defaultAnonymous,
	} {
		if err := bag.Set(path, value); err != nil {
			return err
		}
	}
	return nil
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(disk.DiskCreatorGroup)).
		Register()
}
