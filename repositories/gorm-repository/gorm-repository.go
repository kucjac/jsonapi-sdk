package gormrepo

import (
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db/gormconv"
	"reflect"
	"runtime/debug"
)

const (
	annotationBelongsTo  = "belongs_to"
	annotationHasOne     = "has_one"
	annotationManyToMany = "many_to_many"
	annotationHasMany    = "has_many"
)

var (
	IErrBadRelationshipField = errors.New("This repository does not allow relationship filter of field different than primary.")
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
		// fmt.Println(err.Error())
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
) (err error) {
	var (
		fieldScope *gorm.Scope
		gormField  *gorm.StructField
		fkField    *gorm.Field

		getDBRelationship = func(singleValue, relationValue reflect.Value) error {
			// fmt.Println("DBRelationship")
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
			// fmt.Println("BelongsToRelationship")
			relationPrimary := relationValue.Elem().FieldByIndex(fieldScope.PrimaryField().Struct.Index)
			fkValue := singleValue.Elem().FieldByIndex(fkField.Struct.Index)
			relationPrimary.Set(fkValue)
		}

		// funcs
		getRelationshipSingle = func(singleValue reflect.Value) error {
			// fmt.Println("RelationshipSingle")
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
				// fmt.Printf("GetDBRel Err: %v\n", err)
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
	// fmt.Printf("Get relationship: '%s' -> '%s'\n", scope.Struct.GetCollectionType(), field.GetFieldName())

	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			switch perr := r.(type) {
			case *reflect.ValueError:
				err = fmt.Errorf("Provided invalid value input to the repository. Error: %s", perr.Error())
			case error:
				err = perr
			case string:
				err = errors.New(perr)
			default:
				err = fmt.Errorf("Unknown panic occured during getting scope's relationship.")
			}
		}
	}()

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
	if v.Kind() == reflect.Slice {
		// there would be more than one value

		// if v.Kind() != reflect.Slice {
		// 	err = fmt.Errorf("Invalid value type provided. '%v'", v.Type())
		// 	return err
		// }
		// fmt.Println("Get multiple relationships Single")
		for i := 0; i < v.Len(); i++ {

			singleValue := v.Index(i)
			err = getRelationshipSingle(singleValue)
			if err != nil {
				return err
			}
		}
	} else {
		// fmt.Printf("Get Single relationship: '%+v', '%v'", v.Kind())
		err = getRelationshipSingle(v)
		if err != nil {
			return err
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
			fmt.Println("Should be true")
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
			// fmt.Println("Rel")
			// not implemented yet.
			// it should order the relationship id
			// and then make
		}
	}

	return nil
}
