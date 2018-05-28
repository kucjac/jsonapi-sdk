package jsonapisdk

import (
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
	"github.com/kucjac/uni-logger"
	"golang.org/x/text/language"
	"net/http"
	"reflect"
	"strconv"
)

type JSONAPIHandler struct {
	// jsonapi controller
	Controller *jsonapi.Controller

	// Logger
	log unilogger.ExtendedLeveledLogger

	// Repositories
	Repositories      map[reflect.Type]Repository
	DefaultRepository Repository

	// DBErrMgr database error manager
	DBErrMgr *ErrorManager

	// Supported Languages
	SupportedLanguages []language.Tag

	// LanguageMatcher matches the possible language
	LanguageMatcher language.Matcher
}

func NewHandler(
	c *jsonapi.Controller,
	log unilogger.ExtendedLeveledLogger,
	DBErrMgr *ErrorManager,
) *JSONAPIHandler {
	if DBErrMgr == nil {
		DBErrMgr = NewDBErrorMgr()
	}
	return &JSONAPIHandler{
		Controller:   c,
		log:          log,
		Repositories: make(map[reflect.Type]Repository),
		DBErrMgr:     DBErrMgr,
	}
}

// SetLanguages sets the default langauges for given handler.
// Creates the language matcher for given languages.
func (h *JSONAPIHandler) SetLanguages(languages ...language.Tag) {
	h.LanguageMatcher = language.NewMatcher(languages)
	h.SupportedLanguages = languages
}

func (h *JSONAPIHandler) SetDefaultRepo(Repositoriesitory Repository) {
	h.DefaultRepository = Repositoriesitory
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

func (h *JSONAPIHandler) MarshalScope(
	scope *jsonapi.Scope,
	rw http.ResponseWriter,
	req *http.Request,
) {
	payload, err := h.Controller.MarshalScope(scope)
	if err != nil {
		h.errMarshalScope(err, scope.Struct.GetType(), rw, req)
		return
	}
	if err = jsonapi.MarshalPayload(rw, payload); err != nil {
		h.errMarshalPayload(payload, err, scope.Struct.GetType(), rw, req)
		return
	}
}

func (h *JSONAPIHandler) UnmarshalScope(
	model interface{},
	rw http.ResponseWriter,
	req *http.Request,
) *jsonapi.Scope {
	scope, errObj, err := jsonapi.UnmarshalScopeOne(req.Body, h.Controller)
	if err != nil {
		h.log.Errorf("Error while unmarshaling: '%v' for path: '%s' and method: %s. Error: %s.", reflect.TypeOf(model), req.URL.Path, req.Method, err)
		h.MarshalInternalError(rw)
		return nil
	}

	if errObj != nil {
		h.MarshalErrors(rw, errObj)
		return nil
	}

	if t := reflect.TypeOf(model).Elem(); scope.Struct.GetType() != t {
		h.log.Errorf("Model and the path collection does not match for path: '%s' and method: '%s' for model: %v", req.URL.Path, req.Method, t)
		h.MarshalInternalError(rw)
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

func SetContentType(rw http.ResponseWriter) {
	rw.Header().Set("Content-Type", jsonapi.MediaType)
}

func (h *JSONAPIHandler) getModelRepositoryByType(modelType reflect.Type) Repository {
	repo, ok := h.Repositories[modelType]
	if !ok {
		repo = h.DefaultRepository
	}
	return repo
}

// Exported method to get included values for given scope
func (h *JSONAPIHandler) GetIncluded(
	scope *jsonapi.Scope,
	rw http.ResponseWriter,
	req *http.Request,
	tag language.Tag,
) (correct bool) {
	// if the scope is the root and there is no included scopes return fast.
	if scope.IsRoot() && len(scope.IncludedScopes) == 0 {
		return true
	}

	if err := scope.SetCollectionValues(); err != nil {
		h.log.Errorf("Setting collection values for the scope of type: %v. Err: %v", scope.Struct.GetType(), err)
		h.MarshalInternalError(rw)
		return
	}
	// h.log.Debugf("After setting collection values for: %v", scope.Struct.GetType())

	// h.log.Debug(scope.GetCollectionScope().IncludedValues)

	for scope.NextIncludedField() {
		includedField, err := scope.CurrentIncludedField()
		if err != nil {
			h.log.Error(err)
			h.MarshalInternalError(rw)
			return
		}

		missing, err := includedField.GetMissingPrimaries()
		if err != nil {
			h.log.Errorf("While getting missing objects for: '%v'over included field an error occured: %v", includedField.GetFieldName(), err)
			h.MarshalInternalError(rw)
			return
		}

		if len(missing) > 0 {
			includedField.Scope.SetIDFilters(missing...)
			if includedField.Scope.UseI18n() {
				includedField.Scope.SetLanguageFilter(tag.String())
			}
			includedRepo := h.GetModelRepositoryByType(includedField.Scope.Struct.GetType())

			// Get NewMultipleValue
			includedField.Scope.NewValueMany()
			dbErr := includedRepo.List(includedField.Scope)
			if dbErr != nil {
				h.manageDBError(rw, dbErr)
				return
			}

			if correct = h.GetIncluded(includedField.Scope, rw, req, tag); !correct {
				return
			}
		}
	}
	scope.ResetIncludedField()
	return true
}

func (h *JSONAPIHandler) manageDBError(rw http.ResponseWriter, dbErr *unidb.Error) {
	errObj, err := h.DBErrMgr.Handle(dbErr)
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
	h.log.Errorf("Error while setting id filter for the path: '%s', and scope: of type '%v'. Error: %v", req.URL.Path, scope.Struct.GetType(), err)
	h.MarshalInternalError(rw)
	return
}

func (h *JSONAPIHandler) errMarshalPayload(
	payload jsonapi.Payloader,
	err error,
	model reflect.Type,
	rw http.ResponseWriter,
	req *http.Request,
) {
	h.log.Errorf("Error while marshaling payload: '%v'. For model: '%v', Path: '%s', Method: '%s', Error: %v", payload, model, req.URL.Path, req.Method, err)
	h.MarshalInternalError(rw)
}

func (h *JSONAPIHandler) errMarshalScope(
	err error,
	model reflect.Type,
	rw http.ResponseWriter,
	req *http.Request,
) {
	h.log.Errorf("Error while marshaling scope for model: '%v', for path: '%s', and method: '%s', Error: %s", model, req.URL.Path, req.Method, err)
	h.MarshalInternalError(rw)
}
