package jsonapisdk

import (
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
)

// Repository is an interface that specifies
type Repository interface {
	Create(*jsonapi.Scope) *unidb.Error
	Get(*jsonapi.Scope) *unidb.Error
	List(*jsonapi.Scope) *unidb.Error
	Patch(*jsonapi.Scope) *unidb.Error
	Delete(*jsonapi.Scope) *unidb.Error
}
