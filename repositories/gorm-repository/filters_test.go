package gormrepo

import (
	"github.com/kucjac/jsonapi"
	"testing"
)

func TestBuildRelationshipFilter(t *testing.T) {
	c := prepareJSONAPI(blogModels...)
	repo := prepareGORMRepo(blogModels...)

	rw, req := getHttpPair("GET", "/blogs?filter[blogs][post][id]", nil)
	scope := c.BuildScopeList(req, &Blog{})

}
