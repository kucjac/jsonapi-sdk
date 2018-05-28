package gormrepo

import (
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
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
