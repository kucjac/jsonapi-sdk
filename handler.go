package jsonapisdk

import (
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
	"github.com/kucjac/uni-logger"
	"net/http"
	"reflect"
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
			jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
			return
		}
		if errs != nil {
			jsonapi.MarshalErrors(rw, errs...)
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
			jsonapi.MarshalErrors(rw, errs...)
			return
		}
		if len(errs) > 0 {
			jsonapi.MarshalErrors(rw, errs...)
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
		scope := h.UnmarshalScope(model, rw, req)
		if scope == nil {
			return
		}
		SetJSONAPIType(rw)

		// Set the ID for given model's scope
		errs, err := h.controller.SetIDFilter(req, scope)
		if err != nil {
			h.errSetIDFilter(scope, err, rw, req)
			return
		}
		if len(errs) > 0 {
			jsonapi.MarshalErrors(rw, errs...)
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
			jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
			return
		}

		// Set the ID for given model's scope
		errs, err := h.controller.SetIDFilter(req, scope)
		if err != nil {
			h.errSetIDFilter(scope, err, rw, req)
			return
		}
		if len(errs) > 0 {
			jsonapi.MarshalErrors(rw, errs...)
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
		jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
		return nil
	}
	if errObj != nil {
		jsonapi.MarshalErrors(rw, errObj)
		return nil
	}
	return scope
}

func (h *JSONAPIHandler) manageDBError(rw http.ResponseWriter, dbErr *unidb.Error) {
	h.log.Info(dbErr)
	errObj, err := h.dbErrMgr.Handle(dbErr)
	if err != nil {
		h.log.Error(dbErr.Message)
		jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
		return
	}
	jsonapi.MarshalErrors(rw, errObj)
	return
}

func (h *JSONAPIHandler) errSetIDFilter(
	scope *jsonapi.Scope,
	err error,
	rw http.ResponseWriter,
	req *http.Request,
) {
	h.log.Errorf("Error while setting id filter for the path: '%s', and scope: '%+v'. Error: %v", req.URL.Path, scope, err)
	jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
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
	jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
}

func (h *JSONAPIHandler) errMarshalScope(
	model interface{},
	err error,
	rw http.ResponseWriter,
	req *http.Request,
) {
	h.log.Errorf("Error while marshaling scope for model: '%v', for path: '%s', and method: '%s', Error: %s", reflect.TypeOf(model), req.URL.Path, req.Method, err)
	jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
}

func SetJSONAPIType(rw http.ResponseWriter) {
	rw.Header().Set("Content-Type", jsonapi.MediaType)
}
