package flam

import (
	"errors"

	"gorm.io/gorm"
)

type migrationDao struct{}

func newMigrationDao(
	connection DatabaseConnection,
) (*migrationDao, error) {
	d := &migrationDao{}
	if e := connection.AutoMigrate(&migrationRecord{}); e != nil {
		return nil, e
	}

	return d, nil
}

func (dao migrationDao) List(
	connection DatabaseConnection,
) ([]migrationRecord, error) {
	var models []migrationRecord
	result := connection.Order("created_at desc").Find(&models)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}

	return models, nil
}

func (dao migrationDao) Last(
	connection DatabaseConnection,
) (*migrationRecord, error) {
	model := &migrationRecord{}
	result := connection.Order("created_at desc").FirstOrInit(model, migrationRecord{Version: ""})
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}

	return model, nil
}

func (dao migrationDao) Up(
	connection DatabaseConnection,
	version string,
	description string,
) (*migrationRecord, error) {
	model := &migrationRecord{Version: version, Description: description}
	result := connection.Create(model)
	if result.Error != nil {
		return nil, result.Error
	}

	return model, nil
}

func (dao migrationDao) Down(
	connection DatabaseConnection,
	last *migrationRecord,
) error {
	return connection.Delete(last).Error
}
