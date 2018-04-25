package repositories

import (
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
	"github.com/kucjac/uni-db/gormconv"
	"reflect"
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
) (db *gorm.DB, err error) {
	gormScope := g.db.NewScope(scope.Value)
	mStruct := gormScope.GetModelStruct()
	var columnName string
	var operator string

	// every filter field is new search.Where
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

		default:
			err = fmt.Errorf("Unsupported jsonapi field type: '%v' for field: '%s' in model: '%v'.", filter.Field.GetJSONAPIType(), filter.Field.GetFieldName(), scope.Struct.GetType())
			return nil, err
		}

		if gormField == nil {
			err = fmt.Errorf("Invalid filtering field: '%v' not found in the gorm ModelStruct: '%v'", filter.Field.GetFieldName(), mStruct.ModelType)
			return nil, err
		}
		columnName = gormField.DBName
		if filter.Field.GetJSONAPIType() == jsonapi.Attribute || filter.Field.GetJSONAPIType() == jsonapi.Primary {
			err := addQueryToScope(filter, gormScope)
		} else {
			// no direct getter for table name
			tempScope := gormScope.New(reflect.New(filter.Field.GetRelatedModelType()))
			tempScope.SelectAttrs
			for _, rel := range filter.Relationships {
				err := addQueryToScope(rel, tempScope)
				if err != nil {
					return err
				}
			}
			tempScope.Search.Select(tempScope.PrimaryField().DBName)
			expr := tempScope.DB().QueryExpr()
			gormScope.Search.Where(columnName, expr)
		}
	}
	return gormScope.DB(), nil
}

func addQueryToScope(filter *jsonapi.FilterScope, scope *gorm.Scope) error {
	for _, fv := range filter.Values {
		op := sqlizeOperator(fv.Operator)
		var valueMark string
		if fv.Operator == jsonapi.OpIn || fv.Operator == jsonapi.OpNotIn {
			valueMark = "(?)"
		} else {
			if len(fv.Values) > 1 {
				err = fmt.Errorf("Too many values for given operator: '%s', '%s'", fv.Values, fv.Operator)
				return err
			}
			valueMark = "?"
		}
		q := fmt.Sprintf("%s %s %s", columnName, op, valueMark)
		scope.Search.Where(q, fv.Values...)
	}
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
