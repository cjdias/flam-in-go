package tests

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func SetupFactoryConfig(
	app flam.Application,
	ctrl *gomock.Controller,
) *mocks.MockFactoryConfig {
	factoryConfig := mocks.NewMockFactoryConfig(ctrl)
	if e := app.Container().Provide(func() flam.FactoryConfig { return factoryConfig }); e != nil {
		_ = app.Container().Decorate(func(flam.FactoryConfig) flam.FactoryConfig { return factoryConfig })
	}

	return factoryConfig
}

func SetupDatabase() (*gorm.DB, sqlmock.Sqlmock) {
	db, mock, _ := sqlmock.New()

	dialect := mysql.New(mysql.Config{
		DSN:                       "sqlmock_db_0",
		DriverName:                "mysql",
		Conn:                      db,
		SkipInitializeWithVersion: true})

	conn, _ := gorm.Open(dialect, &gorm.Config{
		Logger: logger.Discard})

	return conn, mock
}
