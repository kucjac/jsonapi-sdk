package jsonapisdk

import (
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
	"github.com/kucjac/uni-logger"
	"net/http"
	"reflect"
	"strconv"
)

type JSONAPIHandler struct {
	controller        *jsonapi.Controller
	log               unilogger.ExtendedLeveledLogger
	repos             map[reflect.Type]Repository
	defaultRepository Repository
	dbErrMgr          *ErrorManager
}

func NewHandler(
	c *jsonapi.Controller,
	log unilogger.ExtendedLeveledLogger,
	dbErrMgr *ErrorManager,
) *JSONAPIHandler {
	if dbErrMgr == nil {
		dbErrMgr = NewDBErrorMgr()
	}
	return &JSONAPIHandler{
		controller: c,
		log:        log,
		repos:      make(map[reflect.Type]Repository),
		dbErrMgr:   dbErrMgr,
	}
}

func (h *JSONAPIHandler) SetDefaultRepo(repository Repository) {
	h.defaultRepository = repository
}

func (h *JSONAPIHandler) GetModelsRepository(model interface{}) Repository {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return h.getModelRepositoryByType(t)
}

func (h *JSONAPIHandler) GetModelRepositoryByType(modelType reflect.Type) Repository {
	return h.getModelRepositoryByType(modelType)
}

func (h *JSONAPIHandler) getModelRepositoryByType(modelType reflect.Type) Repository {
	repo, ok := h.repos[modelType]
	if !ok {
		repo = h.defaultRepository
	}
	return repo
}

func (h *JSONAPIHandler) Create(model interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		SetJSONAPIType(rw)
		scope := h.UnmarshalScope(model, rw, req)
		if scope == nil {
			return
		}

		repo := h.GetModelsRepository(model)
		if dbErr := repo.Create(scope); dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}
		h.MarshalScope(scope, rw, req, model)
		rw.WriteHeader(http.StatusCreated)
	}
}

func (h *JSONAPIHandler) Get(model interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		SetJSONAPIType(rw)
		scope, errs, err := h.controller.BuildScopeSingle(req, model)
		if err != nil {
			h.log.Error(err)
			h.MarshalInternalError(rw)
			return
		}
		if errs != nil {
			h.MarshalErrors(rw, errs...)
			return
		}

		repo := h.GetModelsRepository(model)
		dbErr := repo.Get(scope)
		if dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

		for _, includedScope := range scope.IncludedScopes {
			if len(includedScope.IncludeValues) > 0 {
				includeRepo := h.getModelRepositoryByType(includedScope.Struct.GetType())
				if dbErr = includeRepo.List(includedScope); dbErr != nil {
					h.manageDBError(rw, dbErr)
					return
				}
			}
		}

		// get included
		h.MarshalScope(scope, rw, req, model)
		return
	}
}

func (h *JSONAPIHandler) List(model interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		SetJSONAPIType(rw)
		scope, errs, err := h.controller.BuildScopeList(req, model)
		if err != nil {
			h.log.Error(err)
			h.MarshalInternalError(rw)
			return
		}
		if len(errs) > 0 {
			h.MarshalErrors(rw, errs...)
			return
		}

		repo := h.GetModelsRepository(model)

		dbErr := repo.List(scope)
		if dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}
		// get included

		for _, includedScope := range scope.IncludedScopes {
			if len(includedScope.IncludeValues) > 0 {
				includedRepo := h.GetModelRepositoryByType(scope.Struct.GetType())
				if dbErr = includedRepo.List(includedScope); dbErr != nil {
					h.manageDBError(rw, dbErr)
					return
				}
			}
		}

		h.MarshalScope(scope, rw, req, model)

		return
	}
}

// func (h *JSONAPIHandler) GetRelated(root interface{}) http.HandlerFunc {
// 	return func(rw http.ResponseWriter, req *http.Request) {

// 	}
// }

func (h *JSONAPIHandler) Patch(model interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		// UnmarshalScope from the request body.
		SetJSONAPIType(rw)
		scope := h.UnmarshalScope(model, rw, req)
		if scope == nil {
			return
		}

		// Set the ID for given model's scope
		errs, err := h.controller.SetIDFilter(req, scope)
		if err != nil {
			h.errSetIDFilter(scope, err, rw, req)
			return
		}
		if len(errs) > 0 {
			h.MarshalErrors(rw, errs...)
			return
		}

		// Get the repository for given model
		repo := h.GetModelsRepository(model)

		// Use Patch Method on given model's repository for given scope.
		if dbErr := repo.Patch(scope); dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}
		rw.WriteHeader(http.StatusNoContent)
		return
	}
}

func (h *JSONAPIHandler) Delete(model interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		// Create a scope for given delete handler
		SetJSONAPIType(rw)
		scope, err := h.controller.NewScope(model)
		if err != nil {
			h.log.Errorf("Error while creating scope: '%v'", err)
			h.MarshalInternalError(rw)
			return
		}

		// Set the ID for given model's scope
		errs, err := h.controller.SetIDFilter(req, scope)
		if err != nil {
			h.errSetIDFilter(scope, err, rw, req)
			return
		}
		if len(errs) > 0 {
			h.MarshalErrors(rw, errs...)
			return
		}

		repo := h.GetModelsRepository(model)

		if dbErr := repo.Delete(scope); dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}
		rw.WriteHeader(http.StatusNoContent)
	}
}

func (h *JSONAPIHandler) GetIncluded(scope *jsonapi.Scope) bool {

	// var nestedScopes []*jsonapi.Scope
	// // at first get all includedFields from the first part
	// for _, includedField := range scope.IncludedFields {
	// 	if len(includedField.Scope.IncludeValues) > 0 {
	// 		includedRepo := h.GetModelRepositoryByType(scope.Struct.GetType())
	// 		if dbErr := includedRepo.List(includedScope); dbErr != nil {
	// 			h.manageDBError(rw, dbErr)
	// 			return false
	// 		}
	// 	}
	// 	for _, nestedInclude := range includedField.Scope.IncludedFields {
	// 		if nestedScopes == nil {
	// 			nestedScopes = make([]*jsonapi.Scope, 0)
	// 		}
	// 		nestedInclude.S
	// 	}
	// }

	return true
}

func (h *JSONAPIHandler) MarshalScope(
	scope *jsonapi.Scope,
	rw http.ResponseWriter,
	req *http.Request,
	model interface{},
) {
	payload, err := h.controller.MarshalScope(scope)
	if err != nil {
		h.errMarshalScope(model, err, rw, req)
		return
	}
	if err = jsonapi.MarshalPayload(rw, payload); err != nil {
		h.errMarshalPayload(payload, model, err, rw, req)
		return
	}
}

func (h *JSONAPIHandler) UnmarshalScope(
	model interface{},
	rw http.ResponseWriter,
	req *http.Request,
) *jsonapi.Scope {
	scope, errObj, err := jsonapi.UnmarshalScopeOne(req.Body, h.controller)
	if err != nil {
		h.log.Errorf("Error while unmarshaling: '%v' for path: '%s' and method: %s. Error: %s.", reflect.TypeOf(model), req.URL.Path, req.Method, err)
		h.MarshalInternalError(rw)
		return nil
	}
	if errObj != nil {
		h.MarshalErrors(rw, errObj)
		return nil
	}
	return scope
}

func (h *JSONAPIHandler) MarshalInternalError(rw http.ResponseWriter) {
	rw.WriteHeader(http.StatusInternalServerError)
	jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())

}

func (h *JSONAPIHandler) MarshalErrors(rw http.ResponseWriter, errors ...*jsonapi.ErrorObject) {
	if len(errors) > 0 {
		code, err := strconv.Atoi(errors[0].Status)
		if err != nil {
			h.log.Errorf("Status: '%s', for error: %v cannot be converted into http.Status.", errors[0].Status, errors[0])
			h.MarshalInternalError(rw)
			return
		}
		rw.WriteHeader(code)
	} else {
		rw.WriteHeader(http.StatusBadRequest)
	}
	jsonapi.MarshalErrors(rw, errors...)
}

func (h *JSONAPIHandler) manageDBError(rw http.ResponseWriter, dbErr *unidb.Error) {
	h.log.Info(dbErr)
	errObj, err := h.dbErrMgr.Handle(dbErr)
	if err != nil {
		h.log.Error(dbErr.Message)
		h.MarshalInternalError(rw)
		return
	}
	h.MarshalErrors(rw, errObj)
	return
}

func (h *JSONAPIHandler) errSetIDFilter(
	scope *jsonapi.Scope,
	err error,
	rw http.ResponseWriter,
	req *http.Request,
) {
	h.log.Errorf("Error while setting id filter for the path: '%s', and scope: '%+v'. Error: %v", req.URL.Path, scope, err)
	h.MarshalInternalError(rw)
	return
}

func (h *JSONAPIHandler) errMarshalPayload(
	payload jsonapi.Payloader,
	model interface{},
	err error,
	rw http.ResponseWriter,
	req *http.Request,
) {
	h.log.Errorf("Error while marshaling payload: '%v'. For model: '%v', Path: '%s', Method: '%s', Error: %v", payload, reflect.TypeOf(model), req.URL.Path, req.Method, err)
	h.MarshalInternalError(rw)
}

func (h *JSONAPIHandler) errMarshalScope(
	model interface{},
	err error,
	rw http.ResponseWriter,
	req *http.Request,
) {
	h.log.Errorf("Error while marshaling scope for model: '%v', for path: '%s', and method: '%s', Error: %s", reflect.TypeOf(model), req.URL.Path, req.Method, err)
	h.MarshalInternalError(rw)
}

func SetJSONAPIType(rw http.ResponseWriter) {
	rw.Header().Set("Content-Type", jsonapi.MediaType)
}
