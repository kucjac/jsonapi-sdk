package gormrepo

import (
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
	"github.com/kucjac/uni-db/gormconv"
	"reflect"
)

const (
	annotationBelongsTo  = "belongs_to"
	annotationHasOne     = "has_one"
	annotationManyToMany = "many_to_many"
	annotationHasMany    = "has_many"
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

func (g *GORMRepository) Create(scope *jsonapi.Scope) *unidb.Error {
	err := g.db.Create(scope.GetValueAddress()).Error
	if err != nil {
		return g.converter.Convert(err)
	}
	return nil
}

func (g *GORMRepository) Get(scope *jsonapi.Scope) *unidb.Error {
	if scope.Value == nil {
		scope.NewValueSingle()
	}
	gormScope, err := g.buildScopeGet(scope)
	if err != nil {
		errObj := unidb.ErrInternalError.New()
		errObj.Message = err.Error()
		return errObj
	}

	db := gormScope.DB()

	err = db.First(scope.GetValueAddress()).Error
	if err != nil {
		return g.converter.Convert(err)
	}

	// get relationships
	for _, field := range scope.Fieldset {
		if field.IsRelationship() {
			err := g.getRelationship(field, scope, gormScope)
			if err != nil {
				return g.converter.Convert(err)
			}
		}
	}

	return nil
}

func (g *GORMRepository) List(scope *jsonapi.Scope) *unidb.Error {
	if scope.Value == nil {
		scope.NewValueMany()
	}

	gormScope, err := g.buildScopeList(scope)
	if err != nil {
		errObj := unidb.ErrInternalError.New()
		errObj.Message = err.Error()
		return errObj
	}

	db := gormScope.DB()

	err = db.Find(scope.GetValueAddress()).Error
	if err != nil {
		return g.converter.Convert(err)
	}

	scope.SetValueFromAddressable()

	for _, field := range scope.Fieldset {
		if field.IsRelationship() {
			if err = g.getRelationship(field, scope, gormScope); err != nil {
				return g.converter.Convert(err)
			}
		}
	}

	return nil
}

func (g *GORMRepository) Patch(scope *jsonapi.Scope) *unidb.Error {
	if scope.Value == nil {
		// if no value then error
		dbErr := unidb.ErrInternalError.New()
		dbErr.Message = "No value for patch method."
		return dbErr
	}
	gormScope := g.db.NewScope(scope.Value)
	if err := buildFilters(gormScope.DB(), gormScope.GetModelStruct(), scope); err != nil {
		return g.converter.Convert(err)
	}
	if err := gormScope.DB().Update(scope.GetValueAddress()).Error; err != nil {
		return g.converter.Convert(err)
	}
	return nil
}

func (g *GORMRepository) Delete(scope *jsonapi.Scope) *unidb.Error {
	if scope.Value == nil {
		scope.NewValueSingle()
	}
	gormScope := g.db.NewScope(scope.Value)
	if err := buildFilters(gormScope.DB(), gormScope.GetModelStruct(), scope); err != nil {
		return g.converter.Convert(err)
	}

	if err := gormScope.DB().Delete(scope.GetValueAddress()).Error; err != nil {
		return g.converter.Convert(err)
	}

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

func (g *GORMRepository) buildScopeGet(jsonScope *jsonapi.Scope) (*gorm.Scope, error) {
	gormScope := g.db.NewScope(jsonScope.Value)
	mStruct := gormScope.GetModelStruct()
	db := gormScope.DB()

	err := buildFilters(db, mStruct, jsonScope)
	if err != nil {
		return nil, err
	}

	// FieldSets
	if err = buildFieldSets(db, jsonScope, mStruct); err != nil {
		return nil, err
	}
	return gormScope, nil
}

func (g *GORMRepository) buildScopeList(jsonScope *jsonapi.Scope,
) (gormScope *gorm.Scope, err error) {
	gormScope = g.db.NewScope(jsonScope.Value)
	db := gormScope.DB()

	mStruct := gormScope.GetModelStruct()

	// Filters
	err = buildFilters(db, mStruct, jsonScope)
	if err != nil {
		fmt.Println("Ełłoł")
		return nil, err
	}

	// FieldSets
	if err = buildFieldSets(db, jsonScope, mStruct); err != nil {
		return
	}

	// Paginate
	buildPaginate(db, jsonScope)

	// Order
	if err = buildSorts(db, jsonScope, mStruct); err != nil {
		return
	}

	return gormScope, nil
}

// gets relationship from the database
func (g *GORMRepository) getRelationship(
	field *jsonapi.StructField,
	scope *jsonapi.Scope,
	gormScope *gorm.Scope,
) error {
	var (
		fieldScope        *gorm.Scope
		gormField         *gorm.StructField
		fkField           *gorm.Field
		err               error
		getDBRelationship = func(singleValue, relationValue reflect.Value) error {
			db := g.db.New()
			assoc := db.Model(singleValue.Interface()).
				Select(fieldScope.PrimaryField().DBName).
				Association(field.GetFieldName())

			if err := assoc.Error; err != nil {
				return err
			}

			relation := relationValue.Interface()
			err := assoc.Find(relation).Error
			if err != nil {
				return err
			}

			relationValue = reflect.ValueOf(relation)
			return nil
		}

		getBelongsToRelationship = func(singleValue, relationValue reflect.Value) {
			relationPrimary := relationValue.Elem().FieldByIndex(fieldScope.PrimaryField().Struct.Index)
			fkValue := singleValue.Elem().FieldByIndex(fkField.Struct.Index)
			relationPrimary.Set(fkValue)
		}

		// funcs
		getRelationshipSingle = func(singleValue reflect.Value) error {

			var isSlice bool
			relationValue := singleValue.Elem().Field(field.GetFieldIndex())
			t := field.GetFieldType()
			switch t.Kind() {
			case reflect.Slice:
				relationValue = reflect.New(t)
				isSlice = true
			case reflect.Ptr:
				relationValue = reflect.New(t.Elem())
				if fkField != nil {
					getBelongsToRelationship(singleValue, relationValue)
					singleValue.Elem().Field(field.GetFieldIndex()).Set(relationValue)

					return nil
				}
			}

			if err := getDBRelationship(singleValue, relationValue); err != nil {
				return err
			}
			if isSlice {
				singleValue.Elem().Field(field.GetFieldIndex()).Set(relationValue.Elem())
			} else {
				singleValue.Elem().Field(field.GetFieldIndex()).Set(relationValue)
			}

			return nil
		}
	)

	fieldScope = g.db.NewScope(reflect.New(field.GetFieldType()).Elem().Interface())
	if fieldScope == nil {
		err := fmt.Errorf("Empty gorm scope for field: '%s' and model: '%v'.", field.GetFieldName(), scope.Struct.GetType())
		return err
	}

	// Get gormField as a gorm.StructField for given relationship field
	for _, gField := range gormScope.GetModelStruct().StructFields {
		if gField.Struct.Index[0] == field.GetFieldIndex() {
			gormField = gField
			break
		}
	}

	if gormField == nil {
		err := fmt.Errorf("No gormField for field: '%s'", field.GetFieldName())
		return err
	}

	// If given relationship is of Belongs_to type find a gorm
	if gormField.Relationship != nil && gormField.Relationship.Kind == annotationBelongsTo {
		for _, f := range gormScope.Fields() {
			if f.Name == gormField.Relationship.ForeignFieldNames[0] {
				fkField = f
			}
		}
		if fkField == nil {
			err := fmt.Errorf("No foreign field found for field: '%s'", gormField.Relationship.ForeignFieldNames[0])
			return err
		}
	}

	v := reflect.ValueOf(scope.Value)
	if scope.IsMany {
		// there would be more than one value

		if v.Kind() != reflect.Slice {
			err = fmt.Errorf("Invalid value type provided. '%v'", v.Type())
			return err
		}
		for i := 0; i < v.Len(); i++ {
			singleValue := v.Index(i)
			err = getRelationshipSingle(singleValue)
			if err != nil {
				return err
			}
		}
	} else {
		err = getRelationshipSingle(v)
		if err != nil {
			return err
		}

	}

	return nil
}

func addWhere(db *gorm.DB, columnName string, filter *jsonapi.FilterField) error {
	var err error
	for _, fv := range filter.Values {

		op := sqlizeOperator(fv.Operator)
		var valueMark string
		if fv.Operator == jsonapi.OpIn || fv.Operator == jsonapi.OpNotIn {
			valueMark = "("
			for i := range fv.Values {
				valueMark += "?"
				if i != len(fv.Values)-1 {
					valueMark += ","
				}
			}
			valueMark += ")"
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

func buildFilters(db *gorm.DB, mStruct *gorm.ModelStruct, scope *jsonapi.Scope,
) error {

	var (
		columnName string
		err        error
		gormField  *gorm.StructField
	)

	for _, primary := range scope.PrimaryFilters {
		gormField, err = getGormField(primary, mStruct, true)
		if err != nil {
			return err
		}
		addWhere(db, gormField.DBName, primary)
	}

	for _, attrFilter := range scope.AttributeFilters {
		gormField, err = getGormField(attrFilter, mStruct, false)
		addWhere(db, gormField.DBName, attrFilter)
	}

	for _, relFilter := range scope.RelationshipFilters {
		gormField, err = getGormField(relFilter, mStruct, false)
		if err != nil {
			return err
		}
		// no direct getter for table name
		relScope := db.NewScope(reflect.New(relFilter.GetRelatedModelType()))
		relMStruct := relScope.GetModelStruct()
		relDB := relScope.DB()
		err = buildRelationFilters(relDB, relMStruct, relFilter.Relationships...)
		if err != nil {
			return err
		}
		expr := relDB.Table(relMStruct.TableName(relDB)).Select(relScope.PrimaryField().DBName).QueryExpr()
		*db = *db.Where(columnName, expr)

	}
	return nil
}

func buildRelationFilters(
	db *gorm.DB,
	gormModel *gorm.ModelStruct,
	filters ...*jsonapi.FilterField,
) error {
	var (
		gormField *gorm.StructField
		err       error
	)

	for _, filter := range filters {
		var isPrimary bool
		// get gorm structField
		switch filter.GetFieldKind() {
		case jsonapi.Primary:
			isPrimary = true
		case jsonapi.Attribute, jsonapi.RelationshipSingle, jsonapi.RelationshipMultiple:
			isPrimary = false
		default:
			err = fmt.Errorf("Unsupported jsonapi field type: '%v' for field: '%s' in model: '%v'.", filter.GetFieldKind(), filter.GetFieldName(), gormModel.ModelType)
			return err
		}
		gormField, err = getGormField(filter, gormModel, isPrimary)
		if err != nil {
			return err
		}

		if filter.GetFieldKind() == jsonapi.Attribute || filter.GetFieldKind() == jsonapi.Primary {
			err = addWhere(db, gormField.DBName, filter)
			if err != nil {
				return err
			}
		} else {
			// no direct getter for table name
			relScope := db.NewScope(reflect.New(filter.GetRelatedModelType()))
			relMStruct := relScope.GetModelStruct()
			relDB := relScope.DB()
			err = buildRelationFilters(relDB, relMStruct, filter.Relationships...)
			if err != nil {
				return err
			}
			expr := relDB.Table(relMStruct.TableName(relDB)).Select(relScope.PrimaryField().DBName).QueryExpr()
			*db = *db.Where(gormField.DBName, expr)
		}
	}
	return nil
}

func buildPaginate(db *gorm.DB, jsonScope *jsonapi.Scope) {
	if jsonScope.Pagination != nil {
		limit, offset := jsonScope.Pagination.GetLimitOffset()
		db = db.Limit(limit).Offset(offset)
		*db = *db
	}
	return
}

// buildFieldSets helper for building FieldSets
func buildFieldSets(db *gorm.DB, jsonScope *jsonapi.Scope, mStruct *gorm.ModelStruct) error {

	var (
		fields    string
		foundPrim bool
	)
	// add primary

	for _, gormField := range mStruct.PrimaryFields {
		// fmt.Printf("GormFieldIndex: '%v', JsonAPI: '%v'\n", gormField.Struct.Index[0], jsonScope.Struct.GetPrimaryField().GetFieldIndex())
		if gormField.Struct.Index[0] == jsonScope.Struct.GetPrimaryField().GetFieldIndex() {
			// fmt.Println("Should be true")
			fields += gormField.DBName
			foundPrim = true
			break
		}
	}

	if !foundPrim {
		err := fmt.Errorf("The primary field for the model: '%v' is not found within gorm.ModelStruct", mStruct.ModelType)
		return err
	}

	for _, field := range jsonScope.Fieldset {
		if !field.IsRelationship() {
			index := field.GetFieldIndex()
			for _, gField := range mStruct.StructFields {
				if gField.Struct.Index[0] == index {
					// this is the field
					fields += ", " + gField.DBName
				}
			}
		} else {
			if field.GetFieldKind() == jsonapi.RelationshipSingle {

				for _, gField := range mStruct.StructFields {
					if gField.Struct.Index[0] == field.GetFieldIndex() {
						rel := gField.Relationship

						if rel != nil && rel.Kind == "belongs_to" {
							if rel.ForeignDBNames[0] != "id" {
								fields += ", " + rel.ForeignDBNames[0]
							}
						}
					}
				}
			}

		}
	}
	*db = *db.Select(fields)
	return nil
}

func buildSorts(db *gorm.DB, jsonScope *jsonapi.Scope, mStruct *gorm.ModelStruct) error {

	for _, sort := range jsonScope.Sorts {
		if !sort.IsRelationship() {
			index := sort.GetFieldIndex()
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
				err := fmt.Errorf("Sort field: '%s' not found within model: '%s'", sort.GetFieldName(), mStruct.ModelType)

				return err
			}

			order := sField.DBName

			if sort.Order == jsonapi.DescendingOrder {
				order += " DESC"
			}
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

// // gets the BelongsTo Foreign key withing the rootScope for matching field from FieldSet (fs)
// func getBelongsToFKField(
// 	field *jsonapi.StructField,
// 	fieldScope *gorm.Scope,
// ) (belongsToFK *gorm.Field) {
// 	// relation must be of single type
// 	if field.GetFieldKind() == jsonapi.RelationshipSingle {
// 		for _, relField := range fieldScope.Fields() {
// 			if relField.Struct.Index[0] == field.GetFieldIndex() {

// 				switch relField.Relationship.Kind {
// 				case annotationBelongsTo:
// 				case annotationHasMany:
// 				case annotationHasOne:

// 				}
// 				if relField.Relationship.Kind == "belongs_to" {
// 					for _, rf := range rootScope.Fields() {
// 						if rf.Name == relField.Relationship.ForeignFieldNames[0] {
// 							belongsToFK = rf
// 							return
// 						}
// 					}
// 				}

// 			}
// 		}
// 	}
// 	return
// }

func getGormField(
	filterField *jsonapi.FilterField,
	model *gorm.ModelStruct,
	isPrimary bool,
) (*gorm.StructField, error) {

	if isPrimary {
		if len(model.PrimaryFields) == 1 {
			return model.PrimaryFields[0], nil
		} else {
			for _, prim := range model.PrimaryFields {
				if prim.Struct.Index[0] == filterField.GetFieldIndex() {
					return prim, nil
				}
			}
		}
	} else {
		for _, field := range model.StructFields {
			if field.Struct.Index[0] == filterField.GetFieldIndex() {
				return field, nil
			}
		}
	}

	return nil, fmt.Errorf("Invalid filtering field: '%v' not found in the gorm ModelStruct: '%v'", filterField.GetFieldName(), model.ModelType)
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
