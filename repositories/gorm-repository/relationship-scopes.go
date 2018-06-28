package gormrepo

import (
	"github.com/jinzhu/gorm"
	"github.com/kucjac/jsonapi"
	"reflect"
)

func (g *GORMRepository) prepareRelationshipScopes(scope *jsonapi.Scope) (relationScopes []*gorm.Scope, err error) {
	if scope.Value == nil {
		err = IErrNoValuesProvided
		return
	}
	gormScope := g.db.NewScope(scope.Value)

	for _, field := range gormScope.Fields() {
		if rel := field.Relationship; rel != nil {
			fieldScope := g.db.NewScope(value)
			switch rel.Kind {
			case associationBelongsTo:
				fkField, ok := gormScope.FieldByName(rel.ForeignFieldNames[0])
				if !ok {
					err = IErrBadRelationshipField
					return
				}
			case associationHasOne:
			case associationHasMany:
			case associationManyToMany:

			}
		}

	}

	return
}

func (r *GORMRepository) prepareRelScopeWithValue(scope *jsonapi.Scope) (relationScopes []*gorm.Scope, err error) {

	return
}
