package repositories

import (
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
	"github.com/kucjac/uni-db/gormconv"
	"strings"
)

type GORMRepository struct {
	db        *gorm.DB
	converter *gormconv.GORMConverter
}

type queryValue struct {
	query  string
	values []interface{}
}

func New(db *gorm.DB) (*GORMRepository, error) {
	gormRepo := &GORMRepository{}
	err := gormRepo.initialize(db)
	if err != nil {
		return nil, err
	}
	return gormRepo, nil

}

func (g *GORMRepository) initialize(db *gorm.DB) (err error) {
	if db == nil {
		err = errors.New("Nil pointer as an argument provided.")
		return
	}
	g.db = db

	// Get Error converter
	g.converter, err = gormconv.New(db)
	if err != nil {
		return err
	}
	return nil
}

func (g *GORMRepository) buildScopeQuery(scope *jsonapi.Scope,
) (query string, values []interface{}, err error) {
	gormScope := g.db.NewScope(scope.Value)
	mStruct := gormScope.GetModelStruct()

	var columnName string
	var operator string
	for _, filter := range scope.Filters {
		var gormField *gorm.StructField
		switch filter.Field.GetJSONAPIType() {
		case jsonapi.Primary:
			if len(mStruct.PrimaryFields) != 1 {
				gormField = mStruct.PrimaryFields[0]
			} else {
				for _, prim := range mStruct.PrimaryFields {
					if prim.Struct.Index[0] == filter.Field.GetFieldIndex() {
						gormField = prim
						break
					}
				}
			}
		case jsonapi.Attribute, jsonapi.RelationshipSingle, jsonapi.RelationshipMultiple:
			for _, gField := range mStruct.StructFields {
				if gField.Struct.Index[0] == filter.Field.GetReflectStructField().Index[0] {
					gormField = gField
				} else {
					continue
				}
			}
			// how to build from relationships?
			// it need subscopes
		default:
			err = fmt.Errorf("Unsupported jsonapi field type: '%v' for field: '%s' in model: '%v'.", filter.Field.GetJSONAPIType(), filter.Field.GetFieldName(), scope.Struct.GetType())
			return err
		}

		if gormField == nil {
			err = fmt.Errorf("Invalid filtering field: '%v' not found in the gorm ModelStruct: '%v'", filter.Field.GetFieldName(), mStruct.ModelType)
			return err
		}
		columnName = gormField.DBName
		if len(filter.Values) == 1 {
			switch filter.Values[0].Operator {

			}
		}

		// relacja powinna być subquery

	}
	return nil, nil
}

func sqlizeOperator(operator jsonapi.FilterOperator) string {
	switch operator {
	case jsonapi.OpEqual:
		return "="
	case jsonapi.OpIn:
		return "IN"
	case jsonapi.OpNotEqual:
		return "<>"
	case jsonapi.OpNotIn:
		return "NOT IN"
	case jsonapi.OpGreaterEqual:
		return ">="
	case jsonapi.OpGreaterThan:
		return ">"
	case jsonapi.OpLessEqual:
		return "<="
	case jsonapi.OpLessThan:
		return "<"
	case jsonapi.OpContains, jsonapi.OpStartsWith, jsonapi.OpEndsWith:
		return "LIKE"

	}
}

func (g *GORMRepository) Get(scope *jsonapi.Scope) *unidb.Error {

	return nil
}

func (g *GORMRepository) List(scope *jsonapi.Scope) *unidb.Error {
	return nil
}

func (g *GORMRepository) Create(scope *jsonapi.Scope) *unidb.Error {
	return nil
}

func (g *GORMRepository) Patch(scope *jsonapi.Scope) *unidb.Error {
	return nil
}

func (g *GORMRepository) Delete(scope *jsonapi.Scope) *unidb.Error {
	return nil
}
