package jsonapisdk

import (
	"fmt"
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
	"gopkg.in/go-playground/validator.v9"
	"net/http"
	"reflect"
	"strings"
)

// Create returns http.HandlerFunc that creates new 'model' entity within it's repository.
//
// I18n
// It supports i18n of the model. The language in the request is being checked
// if the value provided is supported by the server. If the match is confident
// the language is converted.
//
// Correctly Response with status '201' Created.
func (h *JSONAPIHandler) Create(model *ModelHandler, endpoint *Endpoint) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[model.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}
		SetContentType(rw)
		scope := h.UnmarshalScope(model.ModelType, rw, req)
		if scope == nil {
			return
		}

		/**

		CREATE: LANGUAGE

		*/
		// if the model is i18n-ready control it's language field value
		if scope.UseI18n() {
			lang, ok := h.CheckValueLanguage(scope, rw)
			if !ok {
				return
			}
			h.HeaderContentLanguage(rw, lang)
		}

		/**

		CREATE: PRESET PAIRS

		*/
		for _, pair := range endpoint.PresetPairs {
			presetScope, presetField := pair.GetPair()
			if pair.Key != nil {
				if !h.getPresetFilter(pair.Key, presetScope, req, model) {
					continue
				}
			}

			values, err := h.GetPresetValues(presetScope, rw)
			if err != nil {
				if hErr := err.(*HandlerError); hErr != nil {
					if hErr.Code == ErrNoValues {
						errObj := jsonapi.ErrInsufficientAccPerm.Copy()
						h.MarshalErrors(rw, errObj)
						return
					}
					if !h.handleHandlerError(hErr, rw) {
						return
					}
				} else {
					h.log.Error(err)
					h.MarshalInternalError(rw)
					return
				}
				continue
			}

			if err := h.PresetScopeValue(scope, presetField, values...); err != nil {
				h.log.Errorf("Cannot preset value while creating model: '%s'.'%s'", model.ModelType.Name(), err)
				h.MarshalInternalError(rw)
				return
			}
		}

		/**

		CREATE: PRESET FILTERS

		*/
		for _, filter := range endpoint.PresetFilters {
			value := req.Context().Value(filter.Key)
			if value != nil {
				if err := h.SetPresetFilterValues(filter.FilterField, value); err != nil {
					h.log.Errorf("Cannot preset value for the Create.PresetFilter. Model %v. Field %v. Value: %v", model.ModelType.Name(), filter.StructField.GetFieldName(), value)
					h.MarshalInternalError(rw)
					return
				}
				if err := h.PresetScopeValue(scope, filter.FilterField, value); err != nil {
					h.log.Errorf("Cannot preset value for the model: '%s'. FilterField: %v. Error: %v", model.ModelType.Name(), filter.GetFieldName(), err)
					h.MarshalInternalError(rw)
					return
				}
			}
		}

		/**

		CREATE: VALIDATE MODEL

		*/

		if err := h.CreateValidator.Struct(scope.Value); err != nil {
			if _, ok := err.(*validator.InvalidValidationError); ok {
				errObj := jsonapi.ErrInvalidJSONFieldValue.Copy()
				h.MarshalErrors(rw, errObj)
				return
			}

			validateErrors, ok := err.(validator.ValidationErrors)
			if !ok || ok && len(validateErrors) == 0 {
				h.log.Debugf("Unknown error type while validating. %v", err)
				h.MarshalErrors(rw, jsonapi.ErrInvalidJSONFieldValue.Copy())
				return
			}

			var errs []*jsonapi.ErrorObject
			for _, verr := range validateErrors {
				tag := verr.Tag()

				var errObj *jsonapi.ErrorObject
				if tag == "required" {
					if verr.Field() == "" {
						errObj = jsonapi.ErrInsufficientAccPerm.Copy()
						h.MarshalErrors(rw, errObj)
						return
					}
					errObj = jsonapi.ErrMissingRequiredJSONField.Copy()
					errObj.Detail = fmt.Sprintf("The field: %s, is required.", verr.Field())
					errs = append(errs, errObj)
					continue
				} else if tag == "isdefault" {
					if verr.Field() == "" {
						errObj = jsonapi.ErrInsufficientAccPerm.Copy()
						h.MarshalErrors(rw, errObj)
						return
					}
					errObj = jsonapi.ErrInvalidJSONFieldValue.Copy()
					errObj.Detail = fmt.Sprintf("The field: '%s' must be empty.", verr.Field())
					errs = append(errs, errObj)
					continue
				} else if strings.HasPrefix(tag, "len") {
					if verr.Field() == "" {
						errObj = jsonapi.ErrInsufficientAccPerm.Copy()
						h.MarshalErrors(rw, errObj)
						return
					}
					errObj = jsonapi.ErrInvalidJSONFieldValue.Copy()
					errObj.Detail = fmt.Sprintf("The value of the field: %s is of invalid length.", verr.Field())
					errs = append(errs, errObj)
					continue
				} else {
					errObj = jsonapi.ErrInvalidJSONFieldValue.Copy()
					if verr.Field() != "" {
						errObj.Detail = fmt.Sprintf("Invalid value for the field: '%s'.", verr.Field())
					}
					errs = append(errs, errObj)
					continue
				}
			}
			h.MarshalErrors(rw, errs...)
			return
		}

		/**

		CREATE: PRECHECK PAIRS

		*/

		for _, pair := range endpoint.PrecheckPairs {
			presetScope, presetField := pair.GetPair()
			if pair.Key != nil {
				if !h.getPrecheckFilter(pair.Key, presetScope, req, model) {
					continue
				}
			}

			values, err := h.GetPresetValues(presetScope, rw)
			if err != nil {
				if hErr := err.(*HandlerError); hErr != nil {
					if hErr.Code == ErrNoValues {
						errObj := jsonapi.ErrInsufficientAccPerm.Copy()
						h.MarshalErrors(rw, errObj)
						return
					}
					if !h.handleHandlerError(hErr, rw) {
						return
					}
				} else {
					h.log.Error(err)
					h.MarshalInternalError(rw)
					return
				}
				continue
			}

			if err := h.SetPresetFilterValues(presetField, values...); err != nil {
				h.log.Error("Cannot preset values to the filter value. %s", err)
				h.MarshalInternalError(rw)
				return
			}

			if err := h.CheckPrecheckValues(scope, presetField); err != nil {
				h.log.Debugf("Precheck value error: '%s'", err)
				if err == IErrValueNotValid {
					errObj := jsonapi.ErrInvalidJSONFieldValue.Copy()
					errObj.Detail = "One of the field values are not valid."
					h.MarshalErrors(rw, errObj)
				} else {
					h.MarshalInternalError(rw)
				}
				return
			}
		}

		/**

		CREATE: PRECHECK FILTERS

		*/

		for _, filter := range endpoint.PrecheckFilters {
			value := req.Context().Value(filter.Key)
			if value != nil {
				if err := h.SetPresetFilterValues(filter.FilterField, value); err != nil {
					h.log.Errorf("Cannot preset value for the Create.PresetFilter. Model %v. Field %v. Value: %v", model.ModelType.Name(), filter.StructField.GetFieldName(), value)
				}

				if err := scope.AddFilterField(filter.FilterField); err != nil {
					h.log.Error(err)
					h.MarshalInternalError(rw)
					return
				}
			}
		}

		/**

		CREATE: RELATIONSHIP FILTERS

		*/
		err := h.GetRelationshipFilters(scope, req, rw)
		if err != nil {
			if hErr := err.(*HandlerError); hErr != nil {
				if !h.handleHandlerError(hErr, rw) {
					return
				}
			} else {
				h.log.Error(err)
				h.MarshalInternalError(rw)
				return
			}
		}

		/**

		CREATE: HOOK BEFORE

		*/

		beforeCreater, ok := scope.Value.(HookBeforeCreator)
		if ok {
			if err = beforeCreater.JSONAPIBeforeCreate(scope); err != nil {
				if errObj, ok := err.(*jsonapi.ErrorObject); ok {
					h.MarshalErrors(rw, errObj)
					return
				}
				h.MarshalInternalError(rw)
				return
			}
		}

		repo := h.GetRepositoryByType(model.ModelType)

		/**

		CREATE: REPOSITORY CREATE

		*/
		if dbErr := repo.Create(scope); dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

		/**

		CREATE: HOOK AFTER

		*/
		afterCreator, ok := scope.Value.(HookAfterCreator)
		if ok {
			if err = afterCreator.JSONAPIAfterCreate(scope); err != nil {
				// the value should not be created?
				if errObj, ok := err.(*jsonapi.ErrorObject); ok {
					h.MarshalErrors(rw, errObj)
					return
				}
				h.log.Debugf("Error in HookAfterCreator: %v", err)
				h.MarshalInternalError(rw)
				return
			}
		}

		rw.WriteHeader(http.StatusCreated)
		h.MarshalScope(scope, rw, req)
	}
}

// Get returns a http.HandlerFunc that gets single entity from the "model's"
// repository.
func (h *JSONAPIHandler) Get(model *ModelHandler, endpoint *Endpoint) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[model.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}
		SetContentType(rw)

		/**

		GET: BUILD SCOPE

		*/
		scope, errs, err := h.Controller.BuildScopeSingle(req, reflect.New(model.ModelType).Interface(), nil)
		if err != nil {
			h.log.Error(err)
			h.MarshalInternalError(rw)
			return
		}

		if errs != nil {
			h.MarshalErrors(rw, errs...)
			return
		}

		/**

		GET: LANGUAGE

		*/
		tag, ok := h.GetLanguage(req, rw)
		if !ok {
			return
		}

		if scope.UseI18n() {
			scope.SetLanguageFilter(tag.String())
		}

		/**

		GET: PRECHECK PAIR

		*/
		if !h.AddPrecheckPairFilters(scope, model, endpoint, req, rw, endpoint.PrecheckPairs...) {
			return
		}

		/**

		GET: PRECHECK FILTERS

		*/

		if !h.AddPrecheckFilters(scope, req, rw, endpoint.PrecheckFilters...) {
			return
		}

		/**

		GET: RELATIONSHIP FILTERS

		*/
		err = h.GetRelationshipFilters(scope, req, rw)
		if err != nil {
			if hErr := err.(*HandlerError); hErr != nil {
				if !h.handleHandlerError(hErr, rw) {
					return
				}
			} else {
				h.log.Error(err)
				h.MarshalInternalError(rw)
				return
			}
		}

		repo := h.GetRepositoryByType(model.ModelType)
		// Set NewSingleValue for the scope
		scope.NewValueSingle()

		/**

		GET: HOOK BEFORE

		*/
		if errObj := h.HookBeforeReader(scope); errObj != nil {
			h.MarshalErrors(rw, errObj)
			return
		}

		/**

		GET: REPOSITORY GET

		*/
		dbErr := repo.Get(scope)
		if dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

		/**

		GET: HOOK AFTER

		*/
		if errObj := h.HookAfterReader(scope); errObj != nil {
			h.MarshalErrors(rw, errObj)
			return
		}

		/**

		GET: GET INCLUDED FIELDS

		*/
		if correct := h.GetIncluded(scope, rw, req, tag); !correct {
			return
		}

		// get included
		h.HeaderContentLanguage(rw, tag)
		h.MarshalScope(scope, rw, req)
		return
	}
}

// GetRelated returns a http.HandlerFunc that returns the related field for the 'root' model
// It prepares the scope rooted with 'root' model with some id and gets the 'related' field from
// url. Related field must be a relationship, otherwise an error would be returned.
// The handler gets the root and the specific related field 'id' from the repository
// and then gets the related object from it's repository.
// If no error occurred an jsonapi related object is being returned
func (h *JSONAPIHandler) GetRelated(root *ModelHandler, endpoint *Endpoint) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[root.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}
		SetContentType(rw)

		/**

		GET RELATED: BUILD SCOPE

		*/
		scope, errs, err := h.Controller.BuildScopeRelated(req, reflect.New(root.ModelType).Interface())
		if err != nil {
			h.log.Errorf("An internal error occurred while building related scope for model: '%v'. %v", reflect.TypeOf(root), err)
			h.MarshalInternalError(rw)
			return
		}
		if len(errs) > 0 {
			h.MarshalErrors(rw, errs...)
			return
		}

		scope.NewValueSingle()

		/**

		GET RELATED: LANGUAGE

		*/

		tag, ok := h.GetLanguage(req, rw)
		if !ok {
			return
		}

		if scope.UseI18n() {
			scope.SetLanguageFilter(tag.String())
		}

		/**

		GET RELATED: PRECHECK PAIR

		*/
		if !h.AddPrecheckPairFilters(scope, root, endpoint, req, rw, endpoint.PrecheckPairs...) {
			return
		}

		/**

		GET RELATED: PRECHECK FILTERS

		*/
		if !h.AddPrecheckFilters(scope, req, rw, endpoint.PrecheckFilters...) {
			return
		}

		/**

		GET RELATED: GET RELATIONSHIP FILTERS

		*/

		err = h.GetRelationshipFilters(scope, req, rw)
		if err != nil {
			if hErr := err.(*HandlerError); hErr != nil {
				if !h.handleHandlerError(hErr, rw) {
					return
				}
			} else {
				h.log.Error(err)
				h.MarshalInternalError(rw)
				return
			}
		}

		// Get root repository
		rootRepository := h.GetRepositoryByType(root.ModelType)
		// Get the root for given id
		// Select the related field inside

		/**

		  GET RELATED: HOOK BEFORE READ

		*/

		if errObj := h.HookBeforeReader(scope); errObj != nil {
			h.MarshalErrors(rw, errObj)
			return
		}

		/**

		GET RELATED: REPOSITORY GET ROOT

		*/
		dbErr := rootRepository.Get(scope)
		if dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

		/**

		  GET RELATED: ROOT HOOK AFTER READ

		*/
		if errObj := h.HookAfterReader(scope); errObj != nil {
			h.MarshalErrors(rw, errObj)
			return
		}

		/**

		GET RELATED: BUILD RELATED SCOPE

		*/

		relatedScope, err := scope.GetRelatedScope()
		if err != nil {
			h.log.Errorf("Error while getting Related Scope: %v", err)
			h.MarshalInternalError(rw)
			return
		}

		// if there is any primary filter
		if relatedScope.Value != nil && len(relatedScope.PrimaryFilters) != 0 {

			relatedRepository := h.GetRepositoryByType(relatedScope.Struct.GetType())
			if relatedScope.UseI18n() {
				relatedScope.SetLanguageFilter(tag.String())
			}

			/**

			  GET RELATED: HOOK BEFORE READER

			*/
			if errObj := h.HookBeforeReader(relatedScope); errObj != nil {
				h.MarshalErrors(rw, errObj)
				return
			}

			// SELECT METHOD TO GET
			if relatedScope.IsMany {
				h.log.Debug("The related scope isMany.")
				dbErr = relatedRepository.List(relatedScope)
			} else {
				h.log.Debug("The related scope isSingle.")
				h.log.Debugf("The value of related scope: %+v", relatedScope.Value)
				h.log.Debugf("Fieldset %+v", relatedScope.Fieldset)
				dbErr = relatedRepository.Get(relatedScope)
			}
			if dbErr != nil {
				h.manageDBError(rw, dbErr)
				return
			}

			/**

			HOOK AFTER READER

			*/
			if errObj := h.HookAfterReader(relatedScope); errObj != nil {
				h.MarshalErrors(rw, errObj)
				return
			}
		}
		h.HeaderContentLanguage(rw, tag)
		h.MarshalScope(relatedScope, rw, req)
		return

	}
}

// GetRelationship returns a http.HandlerFunc that returns in the response the relationship field
// for the root model
func (h *JSONAPIHandler) GetRelationship(root *ModelHandler, endpoint *Endpoint) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[root.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}

		/**

		GET RELATIONSHIP: BUILD SCOPE

		*/
		scope, errs, err := h.Controller.BuildScopeRelationship(req, reflect.New(root.ModelType).Interface())
		if err != nil {
			h.log.Error(err)
			h.MarshalInternalError(rw)
			return
		}
		if len(errs) > 0 {
			h.MarshalErrors(rw, errs...)
			return
		}
		scope.NewValueSingle()

		/**

		  GET RELATIONSHIP: LANGUAGE

		*/
		tag, ok := h.GetLanguage(req, rw)
		if !ok {
			return
		}

		if scope.UseI18n() {
			scope.SetLanguageFilter(tag.String())
		}
		h.HeaderContentLanguage(rw, tag)

		/**

		  GET RELATIONSHIP: PRECHECK PAIR

		*/
		if !h.AddPrecheckPairFilters(scope, root, endpoint, req, rw, endpoint.PrecheckPairs...) {
			return
		}

		/**

		GET RELATIONSHIP: PRECHECK FILTERS

		*/

		if !h.AddPrecheckFilters(scope, req, rw, endpoint.PrecheckFilters...) {
			return
		}

		/**

		GET RELATIONSHIP: GET RELATIONSHIP FILTERS

		*/

		err = h.GetRelationshipFilters(scope, req, rw)
		if err != nil {
			if hErr := err.(*HandlerError); hErr != nil {
				if !h.handleHandlerError(hErr, rw) {
					return
				}
			} else {
				h.log.Error(err)
				h.MarshalInternalError(rw)
				return
			}
		}

		/**

		  GET RELATIONSHIP: ROOT HOOK BEFORE READ

		*/

		if errObj := h.HookBeforeReader(scope); errObj != nil {
			h.MarshalErrors(rw, errObj)
			return
		}

		/**

		  GET RELATIONSHIP: GET ROOT FROM REPOSITORY

		*/

		rootRepository := h.GetRepositoryByType(scope.Struct.GetType())
		dbErr := rootRepository.Get(scope)
		if dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

		/**

		  GET RELATIONSHIP: ROOT HOOK AFTER READ

		*/

		if errObj := h.HookAfterReader(scope); errObj != nil {
			h.MarshalErrors(rw, errObj)
			return
		}

		/**

		  GET RELATIONSHIP: GET RELATIONSHIP SCOPE

		*/
		relationshipScope, err := scope.GetRelationshipScope()
		if err != nil {
			h.log.Errorf("Error while getting RelationshipScope for model: %v. %v", scope.Struct.GetType(), err)
			h.MarshalInternalError(rw)
			return
		}

		/**

		  GET RELATIONSHIP: MARSHAL SCOPE

		*/
		h.MarshalScope(relationshipScope, rw, req)
	}
}

// List returns a http.HandlerFunc that response with the model's entities taken
// from it's repository.
// QueryParameters:
//	- filter - filter parameter must be followed by the collection name within brackets
// 		i.e. '[collection]' and the field scoped for the filter within brackets, i.e. '[id]'
//		i.e. url: http://myapiurl.com/api/blogs?filter[blogs][id]=4
func (h *JSONAPIHandler) List(model *ModelHandler, endpoint *Endpoint) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[model.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}
		SetContentType(rw)

		/**

		  LIST: BUILD SCOPE

		*/
		scope, errs, err := h.Controller.BuildScopeList(req, reflect.New(model.ModelType).Interface())
		if err != nil {
			h.log.Error(err)
			h.MarshalInternalError(rw)
			return
		}
		if len(errs) > 0 {
			h.MarshalErrors(rw, errs...)
			return
		}
		scope.NewValueMany()

		/**

		  LIST: LANGUAGE

		*/
		tag, ok := h.GetLanguage(req, rw)
		if !ok {
			return
		}
		if scope.UseI18n() {
			scope.SetLanguageFilter(tag.String())
		}

		h.HeaderContentLanguage(rw, tag)

		/**

		  LIST: PRECHECK PAIRS

		*/
		if !h.AddPrecheckPairFilters(scope, model, endpoint, req, rw, endpoint.PrecheckPairs...) {
			return
		}

		/**

		  LIST: PRECHECK FILTERS

		*/

		if !h.AddPrecheckFilters(scope, req, rw, endpoint.PrecheckFilters...) {
			return
		}
		/**

		  LIST: GET RELATIONSHIP FILTERS

		*/
		err = h.GetRelationshipFilters(scope, req, rw)
		if err != nil {
			if hErr := err.(*HandlerError); hErr != nil {
				if hErr.Code == ErrNoValues {
					scope.NewValueMany()
					h.MarshalScope(scope, rw, req)
					return
				}
				if !h.handleHandlerError(hErr, rw) {
					return
				}
			} else {
				h.log.Error(err)
				h.MarshalInternalError(rw)
				return
			}
		}

		repo := h.GetRepositoryByType(model.ModelType)

		/**

		  LIST: INCLUDE COUNT

		  Include count into meta data
		*/
		if endpoint.CountList {
			scope.CountList = true
		}

		/**

		  LIST: DEFAULT PAGINATION

		*/
		if endpoint.PresetPaginate != nil && scope.Pagination == nil {
			scope.Pagination = endpoint.PresetPaginate
		}

		/**

		  LIST: DEFAULT SORT

		*/
		if len(endpoint.PresetSort) != 0 {
			scope.Sorts = append(endpoint.PresetSort, scope.Sorts...)
		}

		/**

		  LIST: HOOK BEFORE READER

		*/

		if errObj := h.HookBeforeReader(scope); errObj != nil {
			h.MarshalErrors(rw, errObj)
			return
		}

		/**

		  LIST: LIST FROM REPOSITORY

		*/
		dbErr := repo.List(scope)
		if dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

		/**

		  LIST: HOOK AFTER READ

		*/
		if errObj := h.HookAfterReader(scope); errObj != nil {
			h.MarshalErrors(rw, errObj)
			return
		}

		/**

		  LIST: GET INCLUDED

		*/
		if correct := h.GetIncluded(scope, rw, req, tag); !correct {
			return
		}

		/**

		  LIST: MARSHAL SCOPE

		*/
		h.MarshalScope(scope, rw, req)
		return
	}
}

// Patch the patch endpoint is used to patch given entity.
// It accepts only the models that matches the provided model Handler.
// If the incoming model
// PRESETTING:
//	- Preset values using PresetScope
//	- Precheck values using PrecheckScope
func (h *JSONAPIHandler) Patch(model *ModelHandler, endpoint *Endpoint) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[model.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}
		// UnmarshalScope from the request body.
		SetContentType(rw)

		/**

		  PATCH: UNMARSHAL SCOPE

		*/
		scope := h.UnmarshalScope(model.ModelType, rw, req)
		if scope == nil {
			return
		}

		/**

		  PATCH: GET ID FILTER

		  Set the ID for given model's scope
		*/

		err := h.Controller.GetAndSetIDFilter(req, scope)
		if err != nil {
			h.errSetIDFilter(scope, err, rw, req)
			return
		}

		/**

		  PATCH: LANGAUGE

		*/
		if scope.UseI18n() {
			tag, ok := h.CheckValueLanguage(scope, rw)
			if !ok {
				return
			}
			h.HeaderContentLanguage(rw, tag)
		}

		/**

		  PATCH: PRESET PAIRS

		*/
		for _, presetPair := range endpoint.PresetPairs {
			presetScope, presetField := presetPair.GetPair()
			if presetPair.Key != nil {
				if !h.getPresetFilter(presetPair.Key, presetScope, req, model) {
					continue
				}
			}
			values, err := h.GetPresetValues(presetScope, rw)
			if err != nil {
				if hErr := err.(*HandlerError); hErr != nil {
					if hErr.Code == ErrNoValues {
						errObj := jsonapi.ErrInsufficientAccPerm.Copy()
						h.MarshalErrors(rw, errObj)
						return
					}
					if !h.handleHandlerError(hErr, rw) {
						return
					}
				} else {
					h.log.Error(err)
					h.MarshalInternalError(rw)
					return
				}
				continue
			}

			if err := h.PresetScopeValue(scope, presetField, values...); err != nil {
				h.log.Errorf("Cannot preset value while creating model: '%s'.'%s'", model.ModelType.Name(), err)
				h.MarshalInternalError(rw)
				return
			}
		}

		/**

		  PATCH: PRESET FILTERS

		*/

		if !h.SetPresetFilters(scope, model, req, rw, endpoint.PresetFilters...) {
			return
		}

		/**

		  PATCH: PRECHECK PAIRS

		*/
		if !h.AddPrecheckPairFilters(scope, model, endpoint, req, rw, endpoint.PrecheckPairs...) {
			return
		}

		/**

		  PATCH: PRECHECK FILTERS

		*/

		if !h.AddPrecheckFilters(scope, req, rw, endpoint.PrecheckFilters...) {
			return
		}

		err = h.GetRelationshipFilters(scope, req, rw)
		if err != nil {
			if hErr := err.(*HandlerError); hErr != nil {
				if !h.handleHandlerError(hErr, rw) {
					return
				}
			} else {
				h.log.Error(err)
				h.MarshalInternalError(rw)
				return
			}
		}

		// Get the Repository for given model
		repo := h.GetRepositoryByType(model.ModelType)

		/**

		  PATCH: GET MODIFIED RESULT

		*/
		if endpoint.GetModifiedResult {
			scope.GetModifiedResult = true
		}

		/**

		  PATCH: HOOK BEFORE PATCH

		*/
		if beforePatcher, ok := scope.Value.(HookBeforePatcher); ok {
			if err = beforePatcher.JSONAPIBeforePatch(scope); err != nil {
				if errObj, ok := err.(*jsonapi.ErrorObject); ok {
					h.MarshalErrors(rw, errObj)
					return
				}
				h.log.Errorf("Error in HookBeforePatch for model: %v. Error: %v", model.ModelType.Name(), err)
				h.MarshalInternalError(rw)
				return
			}
		}

		/**

		  PATCH: REPOSITORY PATCH

		*/
		// Use Patch Method on given model's Repository for given scope.
		if dbErr := repo.Patch(scope); dbErr != nil {
			if dbErr.Compare(unidb.ErrNoResult) && endpoint.HasPrechecks() {
				errObj := jsonapi.ErrInsufficientAccPerm.Copy()
				errObj.Detail = "Given object is not available for this account or it does not exists."
				h.MarshalErrors(rw, errObj)
				return
			}
			h.manageDBError(rw, dbErr)
			return
		}

		/**

		  PATCH: HOOK AFTER PATCH

		*/
		if afterPatcher, ok := scope.Value.(HookAfterPatcher); ok {
			if err = afterPatcher.JSONAPIAfterPatch(scope); err != nil {
				if errObj, ok := err.(*jsonapi.ErrorObject); ok {
					h.MarshalErrors(rw, errObj)
					return
				}
				h.log.Errorf("Error in HookAfterPatcher for model: %v. Error: %v", model.ModelType.Name(), err)
				h.MarshalInternalError(rw)
				return
			}
		}

		/**

		  PATCH: MARSHAL RESULT

		*/
		if scope.GetModifiedResult {
			h.MarshalScope(scope, rw, req)
		} else {
			rw.WriteHeader(http.StatusNoContent)
		}
		return
	}
}

func (h *JSONAPIHandler) PatchRelated(model *ModelHandler, endpoint *Endpoint) http.HandlerFunc {
	return h.EndpointForbidden(model, PatchRelated)
}

func (h *JSONAPIHandler) PatchRelationship(model *ModelHandler, endpoint *Endpoint) http.HandlerFunc {
	return h.EndpointForbidden(model, PatchRelationship)
}

func (h *JSONAPIHandler) Delete(model *ModelHandler, endpoint *Endpoint) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[model.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}

		/**

		  DELETE: BUILD SCOPE

		*/
		// Create a scope for given delete handler
		scope, err := h.Controller.NewScope(reflect.New(model.ModelType).Interface())
		if err != nil {
			h.log.Errorf("Error while creating scope: '%v' for model: '%v'", err, reflect.TypeOf(model))
			h.MarshalInternalError(rw)
			return
		}

		/**

		  DELETE: GET ID FILTER

		*/
		// Set the ID for given model's scope
		errs, err := h.Controller.GetSetCheckIDFilter(req, scope)
		if err != nil {
			h.errSetIDFilter(scope, err, rw, req)
			return
		}

		if len(errs) > 0 {
			h.MarshalErrors(rw, errs...)
			return
		}

		/**

		  DELETE: LANGUAGE

		*/
		tag, ok := h.GetLanguage(req, rw)
		if !ok {
			return
		}
		if scope.UseI18n() {
			scope.SetLanguageFilter(tag.String())
		}

		/**

		  DELETE: PRECHECK PAIRS

		*/

		if !h.AddPrecheckPairFilters(scope, model, endpoint, req, rw, endpoint.PrecheckPairs...) {
			return
		}

		/**

		  DELETE: PRECHECK FILTERS

		*/

		if !h.AddPrecheckFilters(scope, req, rw, endpoint.PrecheckFilters...) {
			return
		}
		/**

		  DELETE: GET RELATIONSHIP FILTERS

		*/
		err = h.GetRelationshipFilters(scope, req, rw)
		if err != nil {
			if hErr := err.(*HandlerError); hErr != nil {
				if !h.handleHandlerError(hErr, rw) {
					return
				}
			} else {
				h.log.Error(err)
				h.MarshalInternalError(rw)
				return
			}
		}

		/**

		  DELETE: HOOK BEFORE DELETE

		*/
		scope.NewValueSingle()
		if deleteBefore, ok := scope.Value.(HookBeforeDeleter); ok {
			if err = deleteBefore.JSONAPIBeforeDelete(scope); err != nil {
				if errObj, ok := err.(*jsonapi.ErrorObject); ok {
					h.MarshalErrors(rw, errObj)
					return
				}
				h.log.Errorf("Unknown error in Hook Before Delete. Path: %v. Error: %v", req.URL.Path, err)
				h.MarshalInternalError(rw)
				return
			}
		}

		/**

		  DELETE: REPOSITORY DELETE

		*/
		repo := h.GetRepositoryByType(model.ModelType)
		if dbErr := repo.Delete(scope); dbErr != nil {
			if dbErr.Compare(unidb.ErrNoResult) && endpoint.HasPrechecks() {
				errObj := jsonapi.ErrInsufficientAccPerm.Copy()
				errObj.Detail = "Given object is not available for this account or it does not exists."
				h.MarshalErrors(rw, errObj)
				return
			}
			h.manageDBError(rw, dbErr)
			return
		}

		/**

		  DELETE: HOOK AFTER DELETE

		*/
		if afterDeleter, ok := scope.Value.(HookAfterDeleter); ok {
			if err = afterDeleter.JSONAPIAfterDelete(scope); err != nil {
				if errObj, ok := err.(*jsonapi.ErrorObject); ok {
					h.MarshalErrors(rw, errObj)
					return
				}
				h.log.Errorf("Error of unknown type during Hook After Delete. Path: %v. Error %v", req.URL.Path, err)
				h.MarshalInternalError(rw)
				return
			}
		}

		rw.WriteHeader(http.StatusNoContent)
	}
}
