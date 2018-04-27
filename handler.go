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

func (h *JSONAPIHandler) getRepoForModel(model interface{}) Repository {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	repo, ok := h.repos[t]
	if !ok {
		repo = h.defaultRepository
	}
	return repo
}

func (h *JSONAPIHandler) List(model interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		scope, errs, err := h.controller.BuildScopeMany(req, model)
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
		h.log.Info(reflect.TypeOf(scope.Value))
		repo := h.getRepoForModel(model)

		dbErr := repo.List(scope)
		if dbErr != nil {
			errObj, err := h.dbErrMgr.Handle(dbErr)
			if err != nil {
				h.log.Error(err)
				errObj = jsonapi.ErrInternalError.Copy()
			}
			err = jsonapi.MarshalErrors(rw, errObj)
			if err != nil {
				h.log.Error(err)
			}
			return
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

		repo := h.getRepoForModel(model)
		dbErr := repo.Get(scope)
		if dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
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
		h.log.Error(err)
		jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
		return
	}
	jsonapi.MarshalErrors(rw, errObj)
	return
}