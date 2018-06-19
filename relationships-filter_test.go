package jsonapisdk

import (
	"github.com/kucjac/jsonapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/text/language"
	"reflect"
	"testing"
)

func TestGetRelationshipsFilter(t *testing.T) {
	h := prepareHandler([]language.Tag{language.Polish}, &Human{}, &Pet{})

	rw, req := getHttpPair("GET", "/pets?filter[pets][humans][id][lt]=2", nil)
	scope, errs, err := h.Controller.BuildScopeList(req, &Pet{})
	assert.NoError(t, err)
	assert.Empty(t, errs)

	mockRepo := &MockRepository{}
	h.DefaultRepository = mockRepo

	err = h.GetRelationshipFilters(scope, req, rw)
	assert.NoError(t, err)

	rw, req = getHttpPair("GET", "/humans?filter[humans][pets][legs][gt]=3&filter[humans][pets][id][in]=3,4,5", nil)
	scope, errs, err = h.Controller.BuildScopeList(req, &Human{})
	assert.NoError(t, err)
	assert.Empty(t, errs)

	mockRepo.On("List", mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			relScope := args.Get(0).(*jsonapi.Scope)
			assert.NotEmpty(t, relScope.PrimaryFilters)
			assert.NotEmpty(t, relScope.AttributeFilters)
			assert.Empty(t, relScope.RelationshipFilters)
			assert.Empty(t, relScope.Value)

			v := reflect.ValueOf(relScope.Value)
			assert.Equal(t, reflect.Slice, v.Type().Kind())
			relScope.Value = []*Pet{{ID: 3}, {ID: 4}}
		})
	err = h.GetRelationshipFilters(scope, req, rw)
	assert.NoError(t, err)

	assert.NotEmpty(t, scope.RelationshipFilters)
	assert.NotEmpty(t, scope.RelationshipFilters[0])

	assert.Contains(t, scope.RelationshipFilters[0].Relationships[0].Values[0].Values, 3, 4)
}
