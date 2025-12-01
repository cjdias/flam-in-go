package flam

type DiskCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (Disk, error)
}
