package tests

import (
	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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
