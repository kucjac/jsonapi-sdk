package jsonapisdk

import (
	"errors"
	"fmt"
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
	"github.com/kucjac/uni-logger"
	"golang.org/x/text/language"
	"gopkg.in/go-playground/validator.v9"
	"net/http"
	"reflect"
	"runtime/debug"
	"strconv"
	"time"
)

var (
	IErrScopeNoValue         = errors.New("No value provided within scope.")
	IErrPresetInvalidScope   = errors.New("Pressetting invalid scope value.")
	IErrPresetNoValues       = errors.New("Preset no values")
	IErrInvalidValueType     = errors.New("Trying to preset values of invalid type.")
	IErrInvalidScopeType     = errors.New("Invalid scope type. Available values are slice of pointers to struct or pointer to struct")
	IErrValueNotValid        = errors.New("Value not valid.")
	IErrModelHandlerNotFound = errors.New("Model Handler not found.")
)

type JSONAPIHandler struct {
	// jsonapi controller
	Controller *jsonapi.Controller

	// Logger
	log unilogger.LeveledLogger

	// Repositories
	DefaultRepository Repository

	// DBErrMgr database error manager
	DBErrMgr *ErrorManager

	// Supported Languages
	SupportedLanguages []language.Tag

	// LanguageMatcher matches the possible language
	LanguageMatcher language.Matcher

	// Validators validate given
	CreateValidator *validator.Validate
	PatchValidator  *validator.Validate

	// ModelHandlers
	ModelHandlers map[reflect.Type]*ModelHandler
}

// NewHandler creates new handler on the base of
func NewHandler(
	c *jsonapi.Controller,
	log unilogger.LeveledLogger,
	DBErrMgr *ErrorManager,
) *JSONAPIHandler {
	if DBErrMgr == nil {
		DBErrMgr = NewDBErrorMgr()
	}
	h := &JSONAPIHandler{
		Controller:      c,
		log:             log,
		DBErrMgr:        DBErrMgr,
		ModelHandlers:   make(map[reflect.Type]*ModelHandler),
		CreateValidator: validator.New(),
		PatchValidator:  validator.New(),
	}

	h.CreateValidator.SetTagName("create")
	h.PatchValidator.SetTagName("patch")

	// Register jsonapi name func
	h.CreateValidator.RegisterTagNameFunc(JSONAPITagFunc)
	h.PatchValidator.RegisterTagNameFunc(JSONAPITagFunc)

	return h
}

// AddModelHandlers adds the model handlers for given JSONAPI Handler.
// If there are handlers with the same type the funciton returns error.
func (h *JSONAPIHandler) AddModelHandlers(models ...*ModelHandler) error {
	for _, model := range models {
		if _, ok := h.ModelHandlers[model.ModelType]; ok {
			err := fmt.Errorf("ModelHandler of type: '%s' is already inside the JSONAPIHandler", model.ModelType.Name())
			return err
		}
		h.ModelHandlers[model.ModelType] = model
	}
	return nil
}

func (h *JSONAPIHandler) CheckPrecheckValues(
	scope *jsonapi.Scope,
	filter *jsonapi.FilterField,
) (err error) {
	if scope.Value == nil {
		h.log.Errorf("Provided no value for the scope of type: '%s'", scope.Struct.GetType().Name())
		return IErrScopeNoValue
	}

	checkSingle := func(single reflect.Value) bool {
		field := single.Field(filter.GetFieldIndex())
		if len(filter.Relationships) > 0 {
			relatedIndex := filter.Relationships[0].GetFieldIndex()

			switch filter.GetFieldKind() {
			case jsonapi.RelationshipSingle:
				if field.IsNil() {
					err = IErrPresetNoValues
					return false
				}

				relatedField := field.Field(relatedIndex)
				return h.checkValues(filter.Values[0], relatedField)
			case jsonapi.RelationshipMultiple:
				for i := 0; i < field.Len(); i++ {
					fieldElem := field.Index(i)
					relatedField := fieldElem.Field(relatedIndex)
					if ok := h.checkValues(filter.Values[0], relatedField); !ok {
						return false
					}
				}
				return true
			default:
				h.log.Errorf("Invalid filter field kind for field: '%s'. Within model: '%s'.", filter.GetFieldName(), scope.Struct.GetType().Name())
				err = IErrInvalidValueType
				return false
			}
		} else {
			return h.checkValues(filter.Values[0], field)
		}
	}

	v := reflect.ValueOf(scope.Value)
	if v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			single := v.Index(i)
			if ok := checkSingle(single); !ok {
				return IErrValueNotValid
			}
			if err != nil {
				return
			}
		}
	} else if v.Kind() != reflect.Ptr {
		return IErrInvalidScopeType
	} else {
		v = v.Elem()
		if ok := checkSingle(v); !ok {
			return IErrValueNotValid
		}
	}
	return
}

// GetRepositoryByType returns the repository by provided model type.
// If no modelHandler is found within the jsonapi handler - then the default repository would be
// set.
func (h *JSONAPIHandler) GetRepositoryByType(model reflect.Type) (repo Repository) {
	return h.getModelRepositoryByType(model)
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

	// Iterate over included fields
	for scope.NextIncludedField() {
		// Get next included field
		includedField, err := scope.CurrentIncludedField()
		if err != nil {
			h.log.Error(err)
			h.MarshalInternalError(rw)
			return
		}

		// Get the primaries from the scope.collection primaries
		missing, err := includedField.GetMissingPrimaries()
		if err != nil {
			h.log.Errorf("While getting missing objects for: '%v'over included field an error occured: %v", includedField.GetFieldName(), err)
			h.MarshalInternalError(rw)
			return
		}

		if len(missing) > 0 {
			// h.log.Debugf("There are: '%d' missing values in get Included.", len(missing))
			includedField.Scope.SetIDFilters(missing...)
			// h.log.Debugf("Created ID Filters: '%v'", includedField.Scope.PrimaryFilters)

			if includedField.Scope.UseI18n() {
				includedField.Scope.SetLanguageFilter(tag.String())
			}

			includedRepo := h.GetRepositoryByType(includedField.Scope.Struct.GetType())

			// Get NewMultipleValue
			includedField.Scope.NewValueMany()

			if errObj := h.HookBeforeReader(includedField.Scope); errObj != nil {
				h.MarshalErrors(rw, errObj)
				return
			}

			dbErr := includedRepo.List(includedField.Scope)
			if dbErr != nil {
				h.manageDBError(rw, dbErr)
				return
			}

			if errObj := h.HookAfterReader(includedField.Scope); errObj != nil {
				h.MarshalErrors(rw, errObj)
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

func (h *JSONAPIHandler) EndpointForbidden(
	model *ModelHandler,
	endpoint EndpointType,
) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		mStruct := h.Controller.Models.Get(model.ModelType)
		if mStruct == nil {
			h.log.Errorf("Invalid model provided. The Controller does not contain provided model type within ModelMap. Model: '%s'", model.ModelType)
			h.MarshalInternalError(rw)
			return
		}
		errObj := jsonapi.ErrEndpointForbidden.Copy()
		errObj.Detail = fmt.Sprintf("Server does not allow '%s' operation, at given URI: '%s' for the collection: '%s'.", endpoint.String(), req.URL.Path, mStruct.GetCollectionType())
		h.MarshalErrors(rw, errObj)
	}

}

// MarshalScope is a handler helper for marshaling the provided scope.
func (h *JSONAPIHandler) MarshalScope(
	scope *jsonapi.Scope,
	rw http.ResponseWriter,
	req *http.Request,
) {
	SetContentType(rw)
	payload, err := h.Controller.MarshalScope(scope)
	if err != nil {
		h.log.Errorf("Error while marshaling scope for model: '%v', for path: '%s', and method: '%s', Error: %s", scope.Struct.GetType(), req.URL.Path, req.Method, err)
		h.errMarshalScope(rw, req)
		return
	}

	if err = jsonapi.MarshalPayload(rw, payload); err != nil {
		h.errMarshalPayload(payload, err, scope.Struct.GetType(), rw, req)
		return
	}
	return

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

func (h *JSONAPIHandler) UnmarshalScope(
	model reflect.Type,
	rw http.ResponseWriter,
	req *http.Request,
) *jsonapi.Scope {
	scope, errObj, err := jsonapi.UnmarshalScopeOne(req.Body, h.Controller)
	if err != nil {
		h.log.Errorf("Error while unmarshaling: '%v' for path: '%s' and method: %s. Error: %s.", model, req.URL.Path, req.Method, err)
		h.MarshalInternalError(rw)
		return nil
	}

	if errObj != nil {
		h.MarshalErrors(rw, errObj)
		return nil
	}

	if scope.Struct.GetType() != model {
		// h.log.Errorf("Model and the path collection does not match for path: '%s' and method: '%s' for model: %v", req.URL.Path, req.Method, t)
		// h.MarshalInternalError(rw)
		mStruct := h.Controller.Models.Get(model)
		if mStruct == nil {
			h.log.Errorf("No model found for: '%v' within the controller.", model)
			h.MarshalInternalError(rw)
			return nil
		}
		errObj = jsonapi.ErrInvalidResourceName.Copy()
		errObj.Detail = fmt.Sprintf("Provided resource: '%s' is not proper for this endpoint. This endpoint support '%s' collection.", scope.Struct.GetCollectionType(), mStruct.GetCollectionType())
		h.MarshalErrors(rw, errObj)
		return nil
	}
	return scope
}

func (h *JSONAPIHandler) MarshalInternalError(rw http.ResponseWriter) {
	SetContentType(rw)
	rw.WriteHeader(http.StatusInternalServerError)
	jsonapi.MarshalErrors(rw, jsonapi.ErrInternalError.Copy())
}

func (h *JSONAPIHandler) MarshalErrors(rw http.ResponseWriter, errors ...*jsonapi.ErrorObject) {
	SetContentType(rw)
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

func (h *JSONAPIHandler) HandleValidateError(
	model *ModelHandler,
	err error,
	rw http.ResponseWriter,
) {
	if _, ok := err.(*validator.InvalidValidationError); ok {
		h.log.Debug("Invalid Validation Error")
		h.MarshalInternalError(rw)
	}

	// mStruct, err := h.Controller.GetModelStruct(model)
	// if err != nil {
	// 	h.log.Error("Cannot retrieve model from struct.")
	// 	h.MarshalInternalError(rw)
	// }
	vErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		h.MarshalInternalError(rw)
	}

	var errs []*jsonapi.ErrorObject
	for _, fieldError := range vErrors {
		errObj := jsonapi.ErrInvalidJSONFieldValue.Copy()
		errObj.Detail = fmt.Sprintf("Invalid: '%s' for : '%s'", fieldError.ActualTag(), fieldError.Field())
		errs = append(errs, errObj)
	}
	h.MarshalErrors(rw, errs...)
	return
}

func (h *JSONAPIHandler) addPresetFilter(scope *jsonapi.Scope, filter *jsonapi.FilterField) {
	if len(filter.Relationships) > 0 {
		scope.RelationshipFilters = append(scope.RelationshipFilters, filter)
	} else {
		scope.PrimaryFilters = append(scope.PrimaryFilters, filter)
	}
}

func (h *JSONAPIHandler) checkValues(filterValue *jsonapi.FilterValues, fieldValue reflect.Value) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			h.log.Error("Paniced while checking values. '%s'", r)
		}
		ok = false
	}()
	switch filterValue.Operator {
	case jsonapi.OpIn:
		return checkIn(fieldValue, filterValue.Values...)
	case jsonapi.OpNotIn:
		return checkNotIn(fieldValue, filterValue.Values...)
	case jsonapi.OpEqual:
		return checkEqual(fieldValue, filterValue.Values...)
	case jsonapi.OpNotEqual:
		return !checkEqual(fieldValue, filterValue.Values...)
	case jsonapi.OpLessEqual:
	case jsonapi.OpLessThan:
	case jsonapi.OpGreaterEqual:
	case jsonapi.OpGreaterThan:
	default:
		return false
	}
	return false

}

func checkIn(fieldValue reflect.Value, values ...interface{}) (ok bool) {
	var isTime bool
	if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
		isTime = true
		fieldValue = fieldValue.MethodByName("UnixNano")
	}

	for _, value := range values {
		v := reflect.ValueOf(value)
		if isTime {
			v = v.MethodByName("UnixNano")
		}
		if ok = reflect.DeepEqual(v, fieldValue); ok {
			return
		}
	}
	return
}

func checkNotIn(fieldValue reflect.Value, values ...interface{}) (ok bool) {
	return !checkIn(fieldValue, values...)
}

func checkEqual(fieldValue reflect.Value, values ...interface{}) (ok bool) {
	if len(values) != 1 {
		return false
	}
	v := reflect.ValueOf(values[0])
	if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
		return reflect.DeepEqual(fieldValue.MethodByName("UnixNano"), v.MethodByName("UnixNano"))
	}
	return reflect.DeepEqual(fieldValue, v)
}

func checkLessEqual(fieldValue reflect.Value, values ...interface{}) (ok bool) {
	// for _, value := range values {
	// 	v := reflect.ValueOf(i)
	// }
	return
}

func checkLessThan(fieldValue reflect.Value, values ...interface{}) (ok bool) {
	// var isTime bool
	// if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
	// 	isTime = true
	// 	fieldValue = fieldValue.MethodByName("UnixNano")
	// }

	// switch fieldValue.Kind() {
	// case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:

	// case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:

	// case reflect.Float32, reflect.Float64:

	// case reflect.String:

	// case reflect.Struct:

	// default:
	// 	h.log.Errorf("Invalid field type for compare: '%s'", )
	// }

	// for _, value := range values {
	// 	v := reflect.ValueOf(value)
	// 	if isTime {
	// 		v = v.FieldByName("UnixNano")
	// 	}

	// }
	return
}

func checkGreaterEqual(fieldValue reflect.Value, values ...interface{}) (ok bool) {
	return
}

func checkContains(fieldValue reflect.Value, values ...interface{}) (ok bool) {
	return
}

func (h *JSONAPIHandler) addPresetFilterToPresetScope(
	presetScope *jsonapi.Scope,
	presetFilter *jsonapi.FilterField,
) bool {
	switch presetFilter.GetFieldKind() {
	case jsonapi.Primary:
		presetScope.PrimaryFilters = append(presetScope.PrimaryFilters, presetFilter)
	case jsonapi.Attribute:
		presetScope.AttributeFilters = append(presetScope.AttributeFilters, presetFilter)
	default:
		h.log.Warningf("PrecheckFilter cannot be of reltionship field filter type.")
		return false
	}
	return true
}

func (h *JSONAPIHandler) getModelRepositoryByType(modelType reflect.Type) (repo Repository) {
	model, ok := h.ModelHandlers[modelType]
	if !ok {
		repo = h.DefaultRepository
	} else {
		repo = model.Repository
		if repo == nil {
			repo = h.DefaultRepository
		}
	}
	return repo
}

func (h *JSONAPIHandler) handleHandlerError(hErr *HandlerError, rw http.ResponseWriter) bool {
	switch hErr.Code {
	case ErrInternal:
		h.log.Error(hErr.Error())
		h.log.Error(string(debug.Stack()))
		h.MarshalInternalError(rw)
		return false
	case ErrAlreadyWritten:
		return false
	case ErrBadValues, ErrNoModel, ErrValuePreset:
		h.log.Error(hErr.Error())
		h.MarshalInternalError(rw)
		return false
	case ErrNoValues:
		errObj := jsonapi.ErrResourceNotFound.Copy()
		h.MarshalErrors(rw, errObj)
		return false
	case ErrWarning:
		h.log.Warning(hErr)
		return true
	}
	return true

}

func (h *JSONAPIHandler) manageDBError(rw http.ResponseWriter, dbErr *unidb.Error) {
	errObj, err := h.DBErrMgr.Handle(dbErr)
	if err != nil {
		h.log.Error(dbErr.Message)
		h.MarshalInternalError(rw)
		return
	}

	if proto, _ := dbErr.GetPrototype(); proto == unidb.ErrUnspecifiedError || proto == unidb.ErrInternalError {
		h.log.Error(dbErr)
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
	rw http.ResponseWriter,
	req *http.Request,
) {
	h.MarshalInternalError(rw)
}
