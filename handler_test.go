package jsonapisdk

import (
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
	"github.com/stretchr/testify/assert"
	"net/http/httptest"
	"testing"
)

func TestMarshalScope(t *testing.T) {
	h := prepareHandler(defaultLanguages, blogModels...)

	rw, req := getHttpPair("GET", "/blogs/1/current_post", nil)
	scope, err := h.Controller.NewScope(&Blog{})
	assert.NoError(t, err)

	h.MarshalScope(scope, rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)
}

func TestUnmarshalScope(t *testing.T) {
	h := prepareHandler(defaultLanguages, blogModels...)

	rw, req := getHttpPair("GET", "/blogs", nil)
	h.UnmarshalScope(&Blog{}, rw, req)
	assert.Equal(t, 500, rw.Result().StatusCode)
}

func TestMarshalErrors(t *testing.T) {
	// Case 1:
	// Marshal custom error with invalid status (non http.Status style)
	customError := &jsonapi.ErrorObject{ID: "My custom ID", Status: "Invalid status"}
	h := prepareHandler(defaultLanguages)

	rw := httptest.NewRecorder()
	h.MarshalErrors(rw, customError)

	assert.Equal(t, 500, rw.Result().StatusCode)

	// Case 2:
	// no errors provided
	rw = httptest.NewRecorder()
	h.MarshalErrors(rw)
	assert.Equal(t, 400, rw.Result().StatusCode)
}

func TestManageDBError(t *testing.T) {
	h := prepareHandler(defaultLanguages)
	// Case 1:
	// Having custom error not registered within the error manager.
	customError := &unidb.Error{ID: 30, Title: "My custom DBError"}

	// error not registered in the manager
	rw := httptest.NewRecorder()
	h.manageDBError(rw, customError)
	assert.Equal(t, 500, rw.Result().StatusCode)

}
