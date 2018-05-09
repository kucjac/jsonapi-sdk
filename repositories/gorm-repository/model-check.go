package gormrepo

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/kucjac/jsonapi"
)

func CheckGormModels(db *gorm.DB, c *jsonapi.Controller, models ...interface{}) error {
	for _, model := range models {
		if err := checkModel(model, db, c); err != nil {
			return err
		}
	}
	return nil
}

func checkModel(model interface{}, db *gorm.DB, c *jsonapi.Controller) error {
	scope := db.NewScope(model)
	jsonAPIStruct, err := c.GetModelStruct(model)
	if err != nil {
		return err
	}
	gormStruct := scope.GetModelStruct()

	for _, field := range jsonAPIStruct.GetFields() {
		if field.IsRelationship() {
			for _, gormField := range gormStruct.StructFields {
				if gormField.Struct.Index[0] == field.GetReflectStructField().Index[0] {
					if gormField.Relationship == nil {
						err := fmt.Errorf("Invalid relationship for model: %v in field: %v", gormStruct.ModelType, gormField.Name)
						return err
					}
				}
			}
		}
	}
	return nil

}
