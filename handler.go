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

func (h *JSONAPIHandler) List(model interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {

		scope, errs, err := h.controller.BuildScopeList(req, model)
		if err != nil {
			h.log.Error(err)

			err = jsonapi.MarshalErrors(rw, errs...)
			if err != nil {
				h.log.Fatal(err)
			}
			return
		}
		if len(errs) > 0 {
			jsonapi.MarshalErrors(rw, errs...)
			return
		}

		// h.log.Info(reflect.TypeOf(scope.Value))
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

		payload, err := jsonapi.MarshalScope(scope, h.controller)
		if err != nil {
			h.log.Error(err)
			jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
			return
		}

		err = jsonapi.MarshalPayload(rw, payload)
		if err != nil {
			h.log.Error(err)
			jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
		}
		return
	}
}

func (h *JSONAPIHandler) Get(model interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		scope, errs, err := h.controller.BuildScopeSingle(req, model)
		if err != nil {
			h.log.Error(err)
			err = jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
			if err != nil {
				h.log.Error(err)
			}
			return
		}
		if errs != nil {
			err = jsonapi.MarshalErrors(rw, errs...)
			if err != nil {
				h.log.Error(err)
			}
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
		payload, err := jsonapi.MarshalScope(scope, h.controller)
		if err != nil {
			h.log.Error(err)
			jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
			return
		}

		err = jsonapi.MarshalPayload(rw, payload)
		if err != nil {
			h.log.Error(err)
			jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
		}
		return
	}
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
