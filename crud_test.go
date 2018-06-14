package jsonapisdk

import (
	"bytes"
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
	"github.com/kucjac/uni-logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/text/language"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

type funcScopeMatcher func(*jsonapi.Scope) bool

func TestHandlerCreate(t *testing.T) {
	h := prepareHandler(defaultLanguages, blogModels...)
	mockRepo := &MockRepository{}
	h.SetDefaultRepo(mockRepo)

	rw, req := getHttpPair("POST", "/blogs", h.getModelJSON(&Blog{ID: 1, Lang: "pl", CurrentPost: &Post{ID: 1}}))

	// Case 1:
	// Succesful create.
	mockRepo.On("Create", mock.MatchedBy(
		matchScopeByTypeAndID(Blog{}, 1),
	)).Return(nil)

	h.Create(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 200, rw.Result().StatusCode)

	// Case 2:
	// Duplicated value
	rw, req = getHttpPair("POST", "/blogs",
		h.getModelJSON(&Blog{ID: 2, Lang: "pl", CurrentPost: &Post{ID: 1}}))

	mockRepo.On("Create", mock.MatchedBy(
		matchScopeByTypeAndID(Blog{}, 2),
	)).Return(unidb.ErrUniqueViolation.New())

	h.Create(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 409, rw.Result().StatusCode)

	// Case 3:
	// No language provided error
	rw, req = getHttpPair("POST", "/blogs",
		h.getModelJSON(&Blog{ID: 3, CurrentPost: &Post{ID: 1}}))
	h.Create(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 400, rw.Result().StatusCode)

	// Case 4:
	rw, req = getHttpPair("POST", "/blogs", strings.NewReader(`{"data":{"type":"unknown_collection"}}`))
	h.Create(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 400, rw.Result().StatusCode)
}

func TestHandlerGet(t *testing.T) {
	h := prepareHandler(defaultLanguages, blogModels...)
	mockRepo := &MockRepository{}
	h.SetDefaultRepo(mockRepo)

	// Case 1:
	// Getting an object correctly without accept-language header
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			assert.NotNil(t, arg.LanguageFilters)

			arg.Value = &Blog{ID: 1, Lang: arg.LanguageFilters.Values[0].Values[0].(string),
				CurrentPost: &Post{ID: 1}}
		})

	rw, req := getHttpPair("GET", "/blogs/1", nil)
	h.Get(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)

	// assert content-language is the same
	assert.Equal(t, defaultLanguages[0].String(), rw.Header().Get(headerContentLanguage))
	assert.Equal(t, 200, rw.Result().StatusCode)

	// Case 2:
	// Getting a non-existing object
	mockRepo.On("Get",
		mock.AnythingOfType("*jsonapi.Scope")).Once().Return(unidb.ErrUniqueViolation.New())
	rw, req = getHttpPair("GET", "/blogs/123", nil)
	h.Get(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)

	assert.Equal(t, 409, rw.Result().StatusCode)

	// Case 3:
	// assigning bad url - internal error
	rw, req = getHttpPair("GET", "/blogs", nil)
	h.Get(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)

	assert.Equal(t, 500, rw.Result().StatusCode)

	// Case 4:
	// User input error (i.e. invalid query)
	rw, req = getHttpPair("GET", "/blogs/1?include=nonexisting", nil)
	h.Get(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)

	assert.Equal(t, 400, rw.Result().StatusCode)

	// Case 5:
	// User provided unsupported language
	rw, req = getHttpPair("GET", "/blogs/1", nil)
	req.Header.Add(headerAcceptLanguage, "nonsupportedlang")
	h.Get(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)

	assert.Equal(t, 400, rw.Result().StatusCode)

	// Case 6:
	// Getting Included values with nil Values set.
	// Usually the value is inited to be non nil.
	// Otherwise like here it sends internal.
	rw, req = getHttpPair("GET", "/blogs/1?include=current_post", nil)

	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = &Blog{ID: 1, Lang: h.SupportedLanguages[0].String(),
				CurrentPost: &Post{ID: 3}}
		})

	mockRepo.On("List", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = nil
		})

	h.Get(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)

	precheckPair := h.Controller.BuildPrecheckScope("preset=blogs.current_post.comments&filter[blogs][id][eq]=1", "filter[comments][post][id]")

	h.ModelHandlers[reflect.TypeOf(Comment{})].AddPrecheckPair(precheckPair, Get)

	mockRepo.On("List", mock.Anything).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = []*Blog{{ID: 1, CurrentPost: &Post{ID: 3}}}
		})

	mockRepo.On("List", mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			t.Logf("%v", arg.Fieldset)
			arg.Value = []*Post{{ID: 3, Comments: []*Comment{{ID: 1}, {ID: 3}}}}
		})
	mockRepo.On("List", mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = []*Comment{{ID: 1}, {ID: 3}}
		})

	mockRepo.On("Get", mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			t.Logf("Get filters: '%v'", arg.RelationshipFilters[0].Relationships[0].Values[0].Values)
			arg.Value = []*Comment{{ID: 1, Body: "Some body"}, {ID: 3, Body: "Other body"}}
		})

	rw, req = getHttpPair("GET", "/comments/1", nil)
	h.Get(h.ModelHandlers[reflect.TypeOf(Comment{})]).ServeHTTP(rw, req)

}

func TestHandlerGetRelated(t *testing.T) {
	h := prepareHandler(defaultLanguages, blogModels...)
	mockRepo := &MockRepository{}
	h.SetDefaultRepo(mockRepo)

	// Case 1:
	// Correct related field
	rw, req := getHttpPair("GET", "/blogs/1/current_post", nil)

	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			assert.NotNil(t, arg.LanguageFilters)
			arg.Value = &Blog{ID: 1, Lang: arg.LanguageFilters.Values[0].Values[0].(string),
				CurrentPost: &Post{ID: 1}}
		})
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = &Post{ID: 1, Title: "This title", Comments: []*Comment{{ID: 1}, {ID: 2}}}
		})

	h.GetRelated(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 200, rw.Result().StatusCode)

	// Case 2:
	// Invalid field name
	rw, req = getHttpPair("GET", "/blogs/1/current_invalid_post", nil)
	h.GetRelated(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 400, rw.Result().StatusCode)

	// Case 3:
	// Invalid model's url. - internal
	rw, req = getHttpPair("GET", "/blogs/current_invalid_post", nil)
	h.GetRelated(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)

	// Case 4:
	// invalid language
	rw, req = getHttpPair("GET", "/blogs/1/current_post", nil)
	req.Header.Add(headerAcceptLanguage, "invalid_language")
	h.GetRelated(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 400, rw.Result().StatusCode)

	// Case 5:
	// Root repo dberr
	rw, req = getHttpPair("GET", "/blogs/1/current_post", nil)

	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(unidb.ErrUniqueViolation.New())
	h.GetRelated(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 409, rw.Result().StatusCode)

	// Case 6:
	// No primary filter for scope
	rw, req = getHttpPair("GET", "/blogs/1/current_post", nil)
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			assert.NotNil(t, arg.LanguageFilters)
			arg.Value = &Blog{ID: 1, Lang: arg.LanguageFilters.Values[0].Values[0].(string)}
		})
	h.GetRelated(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 200, rw.Result().StatusCode)

	// Case 7:
	// Get many relateds, correct
	rw, req = getHttpPair("GET", "/posts/1/comments", nil)
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = &Post{ID: 1, Comments: []*Comment{{ID: 1}, {ID: 2}}}

		})
	mockRepo.On("List", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).Run(
		func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = []*Comment{{ID: 1, Body: "First comment"}, {ID: 2, Body: "Second Comment"}}
		})
	h.GetRelated(h.ModelHandlers[reflect.TypeOf(Post{})]).ServeHTTP(rw, req)
	assert.Equal(t, 200, rw.Result().StatusCode)

	// Case 8:
	// Get many relateds, with empty map
	rw, req = getHttpPair("GET", "/posts/1/comments", nil)
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = &Post{ID: 1}
		})
	h.GetRelated(h.ModelHandlers[reflect.TypeOf(Post{})]).ServeHTTP(rw, req)
	assert.Equal(t, 200, rw.Result().StatusCode)

	// Case 9:
	// Provided nil value after getting root from repository - Internal Error
	rw, req = getHttpPair("GET", "/posts/1/comments", nil)
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = nil
		})
	h.GetRelated(h.ModelHandlers[reflect.TypeOf(Post{})]).ServeHTTP(rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)

	// Case 9:
	// dbErr for related
	rw, req = getHttpPair("GET", "/posts/1/comments", nil)
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = &Post{ID: 1, Comments: []*Comment{{ID: 1}}}
		})

	mockRepo.On("List", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(unidb.ErrInternalError.New())
	h.GetRelated(h.ModelHandlers[reflect.TypeOf(Post{})]).ServeHTTP(rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)

	// Case 10:
	// Related field that use language has language filter
	rw, req = getHttpPair("GET", "/authors/1/blogs", nil)
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = &Author{ID: 1, Blogs: []*Blog{{ID: 1}, {ID: 2}, {ID: 5}}}
		})

	mockRepo.On("List", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).Run(
		func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			assert.NotNil(t, arg.LanguageFilters)
		})
	h.GetRelated(h.ModelHandlers[reflect.TypeOf(Author{})]).ServeHTTP(rw, req)
}

func TestGetRelationship(t *testing.T) {
	h := prepareHandler(defaultLanguages, blogModels...)
	mockRepo := &MockRepository{}
	h.SetDefaultRepo(mockRepo)

	// Case 1:
	// Correct Relationship
	rw, req := getHttpPair("GET", "/blogs/1/relationships/current_post", nil)
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = &Blog{ID: 1, CurrentPost: &Post{ID: 1}}
		})

	h.GetRelationship(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 200, rw.Result().StatusCode)

	// Case 2:
	// Invalid url - Internal Error
	rw, req = getHttpPair("GET", "/blogs/relationships/current_post", nil)
	h.GetRelationship(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)

	// Case 3:
	// Invalid field name
	rw, req = getHttpPair("GET", "/blogs/1/relationships/different_post", nil)
	h.GetRelationship(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 400, rw.Result().StatusCode)
	t.Log(rw.Body)

	// Case 4:
	// Bad languge provided
	rw, req = getHttpPair("GET", "/blogs/1/relationships/current_post", nil)
	req.Header.Add(headerAcceptLanguage, "invalid language name")
	h.GetRelationship(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 400, rw.Result().StatusCode)

	// Case 5:
	// Error while getting from root repo
	rw, req = getHttpPair("GET", "/blogs/1/relationships/current_post", nil)
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().
		Return(unidb.ErrNoResult.New())
	h.GetRelationship(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 404, rw.Result().StatusCode)

	// Case 6:
	// Error while getting relationship scope. I.e. assigned bad value type - Internal error
	rw, req = getHttpPair("GET", "/blogs/1/relationships/current_post", nil)
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).Run(func(args mock.Arguments) {
		arg := args[0].(*jsonapi.Scope)
		arg.Value = &Author{ID: 1}
	})
	h.GetRelationship(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)

	// Case 7:
	// Provided empty value for the scope.Value
	rw, req = getHttpPair("GET", "/blogs/1/relationships/current_post", nil)
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).Run(func(args mock.Arguments) {
		arg := args[0].(*jsonapi.Scope)
		arg.Value = nil
	})
	h.GetRelationship(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)

	// Case 8:
	// Provided nil relationship value within scope
	rw, req = getHttpPair("GET", "/blogs/1/relationships/current_post", nil)
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).Run(func(args mock.Arguments) {
		arg := args[0].(*jsonapi.Scope)
		arg.Value = &Blog{ID: 1}
	})
	h.GetRelationship(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 200, rw.Result().StatusCode)

	// Case 9:
	// Provided empty slice in hasMany relationship within scope
	rw, req = getHttpPair("GET", "/authors/1/relationships/blogs", nil)
	mockRepo.On("Get", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).Run(func(args mock.Arguments) {
		arg := args[0].(*jsonapi.Scope)
		arg.Value = &Author{ID: 1}
	})

	h.GetRelationship(h.ModelHandlers[reflect.TypeOf(Author{})]).ServeHTTP(rw, req)
	assert.Equal(t, 200, rw.Result().StatusCode)

}

func TestHandlerList(t *testing.T) {
	h := prepareHandler(defaultLanguages, blogModels...)
	mockRepo := &MockRepository{}
	h.SetDefaultRepo(mockRepo)

	// Case 1:
	// Correct with no Accept-Language header
	rw, req := getHttpPair("GET", "/blogs", nil)

	mockRepo.On("List", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			assert.NotNil(t, arg.LanguageFilters)
			arg.Value = []*Blog{{ID: 1, CurrentPost: &Post{ID: 1}, Lang: arg.LanguageFilters.Values[0].Values[0].(string)}}
		})

	h.List(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 200, rw.Result().StatusCode)
	assert.NotEmpty(t, rw.Header().Get(headerContentLanguage))

	// Case 2:
	// Correct with Accept-Language header

	rw, req = getHttpPair("GET", "/blogs", nil)
	req.Header.Add(headerAcceptLanguage, "pl;q=0.9, en")

	mockRepo.On("List", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			assert.NotNil(t, arg.LanguageFilters)
			arg.Value = []*Blog{}
		})

	h.List(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)

	assert.Equal(t, 200, rw.Result().StatusCode)
	assert.NotEmpty(t, rw.Header().Get(headerContentLanguage))

	// Case 3:
	// User input error on query
	rw, req = getHttpPair("GET", "/blogs?include=invalid", nil)
	h.List(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 400, rw.Result().StatusCode)

	// Case 4:
	// Getting incorrect language

	rw, req = getHttpPair("GET", "/blogs", nil)

	//missspelled language (no '=' sign)
	req.Header.Add(headerAcceptLanguage, "pl;q0.9, en")
	h.List(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 400, rw.Result().StatusCode)

	// Case 5:
	// Internal Error - model not precomputed

	rw, req = getHttpPair("GET", "/models", nil)
	h.List(&ModelHandler{ModelType: reflect.TypeOf(Model{})}).ServeHTTP(rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)

	// Case 6:
	// repository error occurred

	rw, req = getHttpPair("GET", "/blogs", nil)

	mockRepo.On("List", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(unidb.ErrInternalError.New())
	h.List(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)

	// Case 7:
	// Getting includes correctly

	rw, req = getHttpPair("GET", "/blogs?include=current_post", nil)

	mockRepo.On("List", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			assert.NotNil(t, arg.LanguageFilters)
			arg.Value = []*Blog{{ID: 1, CurrentPost: &Post{ID: 1}}}
		})
	mockRepo.On("List", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = []*Post{{ID: 1}}
		})

	h.List(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 200, rw.Result().StatusCode)

	// Case 8:
	// Getting includes with an error
	rw, req = getHttpPair("GET", "/blogs?include=current_post", nil)
	mockRepo.On("List", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			assert.NotNil(t, arg.LanguageFilters)
			arg.Value = []*Blog{{ID: 1, CurrentPost: &Post{ID: 1}}}
		})
	mockRepo.On("List", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(0).(*jsonapi.Scope)
			arg.Value = nil
		})
	h.List(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)
}

// func TestHandlerPatch(t *testing.T) {
// 	h := prepareHandler(defaultLanguages, blogModels...)
// 	mockRepo := &MockRepository{}
// 	h.SetDefaultRepo(mockRepo)

// 	// Case 1:
// 	// Correctly Patched
// 	rw, req := getHttpPair("PATCH", "/blogs/1", h.getModelJSON(&Blog{Lang: "en", CurrentPost: &Post{ID: 2}}))

// 	mockRepo.On("Patch", mock.Anything).Once().Return(nil)
// 	h.Patch(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
// 	assert.Equal(t, 204, rw.Result().StatusCode)

// 	// Case 2:
// 	// Bad model provided for the function  -internal
// 	rw, req = getHttpPair("PATCH", "/blogs/1", h.getModelJSON(&Blog{Lang: "pl"}))

// 	h.Patch(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
// 	assert.Equal(t, 500, rw.Result().StatusCode)

// 	// Case 3:
// 	// Incorrect URL for ID provided - internal
// 	rw, req = getHttpPair("PATCH", "/blogs", h.getModelJSON(&Blog{}))
// 	h.Patch(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
// 	assert.Equal(t, 500, rw.Result().StatusCode)

// 	// Case 4:
// 	// No language provided - user error
// 	rw, req = getHttpPair("PATCH", "/blogs/1", h.getModelJSON(&Blog{CurrentPost: &Post{ID: 2}}))
// 	h.Patch(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
// 	assert.Equal(t, 400, rw.Result().StatusCode)

// 	// Case 5:
// 	// Repository error
// 	rw, req = getHttpPair("PATCH", "/blogs/1", h.getModelJSON(&Blog{Lang: "pl", CurrentPost: &Post{ID: 2}}))
// 	mockRepo.On("Patch", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(unidb.ErrForeignKeyViolation.New())
// 	h.Patch(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
// 	assert.Equal(t, 400, rw.Result().StatusCode)
// 	t.Log(rw.Body)
// }

func TestHandlerDelete(t *testing.T) {
	h := prepareHandler(defaultLanguages, blogModels...)
	mockRepo := &MockRepository{}
	h.SetDefaultRepo(mockRepo)

	// Case 1:
	// Correct delete.
	rw, req := getHttpPair("DELETE", "/blogs/1", nil)
	mockRepo.On("Delete", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(nil)
	h.Delete(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 204, rw.Result().StatusCode)

	// Case 2:
	// Invalid model provided
	// rw, req = getHttpPair("DELETE", "/models/1", nil)
	// h.Delete(h.ModelHandlers[reflect.TypeOf(Model{})]).ServeHTTP(rw, req)
	// assert.Equal(t, 500, rw.Result().StatusCode)

	// Case 3:
	// Invalid url for ID - internal
	rw, req = getHttpPair("DELETE", "/blogs", nil)
	h.Delete(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)

	// Case 4:
	// Invalid ID - user error
	rw, req = getHttpPair("DELETE", "/blogs/stringtype-id", nil)
	h.Delete(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 400, rw.Result().StatusCode)

	// Case 5:
	// Repository error
	rw, req = getHttpPair("DELETE", "/blogs/1", nil)
	mockRepo.On("Delete", mock.AnythingOfType("*jsonapi.Scope")).Once().Return(unidb.ErrIntegrConstViolation.New())
	h.Delete(h.ModelHandlers[reflect.TypeOf(Blog{})]).ServeHTTP(rw, req)
	assert.Equal(t, 400, rw.Result().StatusCode)

}

var (
	defaultLanguages = []language.Tag{language.English, language.Polish}
	blogModels       = []interface{}{&Blog{}, &Post{}, &Comment{}, &Author{}}
)

func getHttpPair(method, target string, body io.Reader,
) (rw *httptest.ResponseRecorder, req *http.Request) {
	req = httptest.NewRequest(method, target, body)
	req.Header.Add("Content-Type", jsonapi.MediaType)
	rw = httptest.NewRecorder()
	return
}

func prepareModelHandlers(models ...interface{}) (handlers []*ModelHandler) {
	for _, model := range models {
		handler, err := NewModelHandler(model, nil, FullCRUD)
		if err != nil {
			panic(err)
		}
		handlers = append(handlers, handler)
	}
	return
}

func prepareHandler(languages []language.Tag, models ...interface{}) *JSONAPIHandler {
	c := jsonapi.New()

	logger := unilogger.MustGetLoggerWrapper(unilogger.NewBasicLogger(os.Stderr, "", log.Ldate))

	h := NewHandler(c, logger, NewDBErrorMgr())
	err := c.PrecomputeModels(models...)
	if err != nil {
		panic(err)
	}

	h.AddModelHandlers(prepareModelHandlers(models...)...)

	h.SetLanguages(languages...)

	return h
}

func matchScopeByType(model interface{}) funcScopeMatcher {
	return func(scope *jsonapi.Scope) bool {
		return isSameType(model, scope)
	}
}

func (h *JSONAPIHandler) getModelJSON(
	model interface{},
) *bytes.Buffer {
	scope, err := h.Controller.NewScope(model)
	if err != nil {
		panic(err)
	}

	scope.Value = model

	payload, err := h.Controller.MarshalScope(scope)
	if err != nil {
		panic(err)
	}
	buf := new(bytes.Buffer)

	if err = jsonapi.MarshalPayload(buf, payload); err != nil {
		panic(err)
	}
	return buf

}

func matchScopeByTypeAndID(model interface{}, id interface{}) funcScopeMatcher {
	return func(scope *jsonapi.Scope) bool {
		if matched := isSameType(model, scope); !matched {
			return false
		}

		if scope.Value == nil {
			return false
		}

		v := reflect.ValueOf(scope.Value)
		if v.Type().Kind() != reflect.Ptr {
			return false
		}

		idIndex := scope.Struct.GetPrimaryField().GetFieldIndex()
		return reflect.DeepEqual(id, v.Elem().Field(idIndex).Interface())
	}
}

func isSameType(model interface{}, scope *jsonapi.Scope) bool {
	return reflect.TypeOf(model) == scope.Struct.GetType()
}
