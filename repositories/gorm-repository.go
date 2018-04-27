package repositories

import (
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
	"github.com/kucjac/uni-db/gormconv"
	"reflect"
)

type GORMRepository struct {
	db        *gorm.DB
	converter *gormconv.GORMConverter
}

func New(db *gorm.DB) (*GORMRepository, error) {
	gormRepo := &GORMRepository{}
	err := gormRepo.initialize(db)
	if err != nil {
		return nil, err
	}
	return gormRepo, nil

}

func (g *GORMRepository) Get(scope *jsonapi.Scope) *unidb.Error {
	db, err := g.buildScopeGet(scope)
	if err != nil {
		errObj := unidb.ErrInternalError.New()
		errObj.Message = err.Error()
		return errObj
	}

	err = db.First(scope.Value).Error
	if err != nil {
		errObj := g.converter.Convert(err)
		errObj.Message = err.Error()
		return errObj
	}

	rootScope := db.NewScope(scope.Value)

	// iterate over fields
	for _, fs := range scope.Fields {
		if fs.IsRelationship() {
			fieldGScope := db.NewScope(reflect.New(fs.GetFieldType()).Elem().Interface())
			belongsToFK := getBelongsToFKField(fs, rootScope)
			err := g.getRelationship(scope.Value, fs, fieldGScope, belongsToFK)
			if err != nil {
				errObj := g.converter.Convert(err)
				errObj.Message = err.Error()
				return errObj
			}
		}
	}
	return nil
}

func (g *GORMRepository) List(scope *jsonapi.Scope) *unidb.Error {
	db, err := g.buildScopeSelect(scope)
	if err != nil {
		errObj := unidb.ErrInternalError.New()
		return errObj
	}

	err = db.Find(scope.Value).Error
	if err != nil {
		return g.converter.Convert(err)
	}
	many, err := scope.GetManyValues()
	if err != nil {
		return nil
	}

	rootScope := db.NewScope(scope.Value)

	for _, fs := range scope.Fields {
		if fs.IsRelationship() {
			fieldGScope := db.NewScope(reflect.New(fs.GetFieldType()).Elem().Interface())
			belongsToFK := getBelongsToFKField(fs, rootScope)
			for _, single := range many {
				g.getRelationship(single, fs, fieldGScope, belongsToFK)
			}
		}
	}

	return nil
}

func (g *GORMRepository) Create(scope *jsonapi.Scope) *unidb.Error {
	err := g.db.Create(&scope.Value).Error
	if err != nil {
		return g.converter.Convert(err)
	}
	return nil
}

func (g *GORMRepository) Patch(scope *jsonapi.Scope) *unidb.Error {
	//
	return nil
}

func (g *GORMRepository) Delete(scope *jsonapi.Scope) *unidb.Error {
	return nil
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

func (g *GORMRepository) buildScopeSelect(jsonScope *jsonapi.Scope,
) (db *gorm.DB, err error) {
	gormScope := g.db.NewScope(jsonScope.Value)
	db = gormScope.DB()
	mStruct := gormScope.GetModelStruct()
	// Filters
	err = buildFilters(db, mStruct, jsonScope.GetFilterScopes()...)
	if err != nil {
		return nil, err
	}

	// FieldSets
	buildFieldSets(db, jsonScope, mStruct)

	// Paginate
	buildPaginate(db, jsonScope)

	// Order
	err = buildSorts(db, jsonScope, mStruct)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (g *GORMRepository) buildScopeGet(jsonScope *jsonapi.Scope) (*gorm.DB, error) {
	gormScope := g.db.NewScope(jsonScope.Value)
	mStruct := gormScope.GetModelStruct()
	db := gormScope.DB()

	err := buildFilters(db, mStruct, jsonScope.GetFilterScopes()...)
	if err != nil {
		return nil, err
	}
	// FieldSets
	buildFieldSets(db, jsonScope, mStruct)

	fmt.Println(db.QueryExpr())

	return db, nil
}

// gets relationship from the database
func (g *GORMRepository) getRelationship(
	single interface{},
	fs *jsonapi.StructField,
	fieldGScope *gorm.Scope,
	belongsToFK *gorm.Field,
) error {
	fVal := reflect.ValueOf(single).Elem()
	toSet := fVal.Field(fs.GetFieldIndex())

	relVal := toSet

	t := fs.GetFieldType()
	switch t.Kind() {
	case reflect.Slice:
		relVal = reflect.New(t)
	case reflect.Ptr:
		relVal = reflect.New(t.Elem())
		if belongsToFK != nil {
			primAssoc := relVal.Elem().FieldByIndex(fieldGScope.PrimaryField().Struct.Index)
			fkVal := fVal.FieldByIndex(belongsToFK.Struct.Index)
			primAssoc.Set(fkVal)
			toSet.Set(relVal)
			return nil
		}
	}

	related := relVal.Interface()

	err := g.db.New().Model(single).Select(fieldGScope.PrimaryField().DBName).
		Association(fs.GetFieldName()).Find(related).Error
	if err != nil {
		return err
	}

	relVal = reflect.ValueOf(related)
	if t.Kind() == reflect.Slice {
		relVal = relVal.Elem()
	}
	toSet.Set(relVal)
	return nil
}

func addWhere(db *gorm.DB, columnName string, filter *jsonapi.FilterScope) error {
	var err error
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
		*db = *db.Where(q, fv.Values...)
	}
	return nil
}

func buildFilters(db *gorm.DB, mStruct *gorm.ModelStruct, filters ...*jsonapi.FilterScope,
) error {

	var (
		columnName string
		err        error
	)

	// every filter field is new search.Where
	for _, filter := range filters {

		var gormField *gorm.StructField
		switch filter.Field.GetJSONAPIType() {
		case jsonapi.Primary:
			if len(mStruct.PrimaryFields) == 1 {
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
			err = fmt.Errorf("Unsupported jsonapi field type: '%v' for field: '%s' in model: '%v'.", filter.Field.GetJSONAPIType(), filter.Field.GetFieldName(), mStruct.ModelType)
			return err
		}

		if gormField == nil {
			err = fmt.Errorf("Invalid filtering field: '%v' not found in the gorm ModelStruct: '%v'", filter.Field.GetFieldName(), mStruct.ModelType)
			return err
		}
		columnName = gormField.DBName
		if filter.Field.GetJSONAPIType() == jsonapi.Attribute || filter.Field.GetJSONAPIType() == jsonapi.Primary {
			err = addWhere(db, columnName, filter)
			if err != nil {
				return err
			}
		} else {
			// no direct getter for table name
			relScope := db.NewScope(reflect.New(filter.Field.GetRelatedModelType()))
			relMStruct := relScope.GetModelStruct()
			relDB := relScope.DB()
			err = buildFilters(relDB, relMStruct, filter.Relationships...)
			if err != nil {
				return err
			}
			expr := relDB.Table(relMStruct.TableName(relDB)).Select(relScope.PrimaryField().DBName).QueryExpr()
			*db = *db.Where(columnName, expr)
		}

	}
	return nil
}

func buildPaginate(db *gorm.DB, jsonScope *jsonapi.Scope) {
	if jsonScope.PaginationScope != nil {
		limit, offset := jsonScope.PaginationScope.GetLimitOffset()
		db = db.Limit(limit).Offset(offset)
		*db = *db
	}
	return
}

func buildFieldSets(db *gorm.DB, jsonScope *jsonapi.Scope, mStruct *gorm.ModelStruct) {

	var fields string

	for _, field := range jsonScope.Fields {
		if !field.IsRelationship() {
			index := field.GetFieldIndex()
			for _, gField := range mStruct.StructFields {
				if gField.Struct.Index[0] == index {
					// this is the field
					if fields == "" {
						fields += gField.DBName
					} else {
						fields += ", " + gField.DBName
					}
				}
			}
		} else {
			if field.GetJSONAPIType() == jsonapi.RelationshipSingle {

				for _, gField := range mStruct.StructFields {
					if gField.Struct.Index[0] == field.GetFieldIndex() {
						rel := gField.Relationship
						if rel != nil {
							if rel.Kind == "belongs_to" {
								if rel.ForeignDBNames[0] != "id" {
									if fields == "" {
										fields += rel.ForeignDBNames[0]
									} else {
										fields += ", " + rel.ForeignDBNames[0]
									}
								}
							}
						}
					}
				}
			}

		}
	}
	*db = *db.Select(fields)

	return
}

func buildSorts(db *gorm.DB, jsonScope *jsonapi.Scope, mStruct *gorm.ModelStruct) error {

	for _, sort := range jsonScope.Sorts {
		if !sort.Field.IsRelationship() {
			index := sort.Field.GetFieldIndex()
			var sField *gorm.StructField
			if index == mStruct.PrimaryFields[0].Struct.Index[0] {
				sField = mStruct.PrimaryFields[0]
			} else {
				for _, gField := range mStruct.StructFields {
					if index == gField.Struct.Index[0] {
						sField = gField
					}
				}
			}
			if sField == nil {
				err := fmt.Errorf("Sort field: '%s' not found within model: '%s'", sort.Field.GetFieldName(), mStruct.ModelType)

				return err
			}

			order := sField.DBName

			if sort.Order == jsonapi.DescendingOrder {
				order += " DESC"
			}
			fmt.Println(order)
			*db = *db.Order(order)
		} else {
			fmt.Println("Rel")
			// not implemented yet.
			// it should order the relationship id
			// and then make
		}
	}

	return nil
}

// gets the BelongsTo Foreign key withing the rootScope for matching field from FieldSet (fs)
func getBelongsToFKField(
	fs *jsonapi.StructField,
	rootScope *gorm.Scope,
) (belongsToFK *gorm.Field) {
	// relation must be of single type
	if fs.GetJSONAPIType() == jsonapi.RelationshipSingle {
		for _, relField := range rootScope.Fields() {
			if relField.Struct.Index[0] == fs.GetFieldIndex() {
				if relField.Relationship != nil {
					if relField.Relationship.Kind == "belongs_to" {
						for _, rf := range rootScope.Fields() {
							if rf.Name == relField.Relationship.ForeignFieldNames[0] {
								belongsToFK = rf
								return
							}
						}
					}
				}
			}
		}
	}
	return
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
	return "="
}
