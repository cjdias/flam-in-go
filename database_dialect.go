package flam

import "gorm.io/gorm"

type DatabaseDialect interface {
	gorm.Dialector
}
