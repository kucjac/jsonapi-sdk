package gormrepo

import (
	"fmt"
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/jsonapi-sdk/repositories"
	"github.com/kucjac/uni-db"
	"reflect"
)

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

	/**

	  GET: HookAfterRead

	*/
	if hookAfterRead, ok := scope.Value.(repositories.HookRepoAfterRead); ok {
		if err := hookAfterRead.RepoAfterRead(gormScope.DB(), scope); err != nil {
			return g.converter.Convert(err)
		}
	}

	return nil
}

func (g *GORMRepository) List(scope *jsonapi.Scope) *unidb.Error {
	if scope.Value == nil {
		scope.NewValueMany()
	}

	/**

	  LIST: BUILD SCOPE LIST

	*/
	gormScope, err := g.buildScopeList(scope)
	if err != nil {
		errObj := unidb.ErrInternalError.New()
		errObj.Message = err.Error()
		return errObj
	}

	db := gormScope.DB()

	/**

	  LIST: GET FROM DB

	*/
	err = db.Find(scope.GetValueAddress()).Error
	if err != nil {
		return g.converter.Convert(err)
	}
	scope.SetValueFromAddressable()

	/**

	  LIST: GET RELATIONSHIPS

	*/
	for _, field := range scope.Fieldset {
		if field.IsRelationship() {
			if err = g.getRelationship(field, scope, gormScope); err != nil {
				return g.converter.Convert(err)
			}
		}
	}

	/**

	  LIST: HOOK AFTER READ

	*/

	if repositories.ImplementsHookAfterRead(scope) {
		fmt.Println("Implements")
		v := reflect.ValueOf(scope.Value)
		for i := 0; i < v.Len(); i++ {
			single := v.Index(i).Interface()

			HookAfterRead, ok := single.(repositories.HookRepoAfterRead)
			if ok {
				if err := HookAfterRead.RepoAfterRead(g.db.NewScope(scope.Value).DB(), scope); err != nil {
					return g.converter.Convert(err)
				}
			}
			v.Index(i).Set(reflect.ValueOf(single))
		}
		scope.Value = v.Interface()
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

	db := gormScope.DB().Update(scope.GetValueAddress())
	if err := db.Error; err != nil {
		return g.converter.Convert(err)
	}

	if db.RowsAffected == 0 {
		return unidb.ErrNoResult.New()
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

	db := gormScope.DB().Delete(scope.GetValueAddress())
	if err := db.Error; err != nil {
		return g.converter.Convert(err)
	}

	if db.RowsAffected == 0 {
		return unidb.ErrNoResult.New()
	}

	return nil
}
