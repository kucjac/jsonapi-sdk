package jsonapisdk

import (
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
)

// Repository is an interface that specifies
type Repository interface {
	Create(scope *jsonapi.Scope) *unidb.Error
	Get(scope *jsonapi.Scope) *unidb.Error
	List(scope *jsonapi.Scope) *unidb.Error
	Patch(scope *jsonapi.Scope) *unidb.Error
	Delete(scope *jsonapi.Scope) *unidb.Error
}
