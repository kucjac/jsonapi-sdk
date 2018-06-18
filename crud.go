package jsonapisdk

import (
	"github.com/kucjac/jsonapi"
	"net/http"
	"reflect"
)

// Create returns http.HandlerFunc that creates new 'model' entity within it's repository.
//
// I18n
// It supports i18n of the model. The language in the request is being checked
// if the value provided is supported by the server. If the match is confident
// the language is converted.
//
// Correctly Response with status '201' Created.
func (h *JSONAPIHandler) Create(model *ModelHandler) http.HandlerFunc {
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

		// Validate struct before pressetting it's value
		if err := h.Validator.Struct(scope.Value); err != nil {
			h.HandleValidateError(model, err, rw)
			return
		}

		// Handle Presets
		for _, pair := range model.Create.Presets {
			values, ok := h.GetPresetValues(pair.Scope, pair.Filter, rw)
			if !ok {
				return
			}

			if err := h.PresetScopeValue(scope, pair.Filter, values...); err != nil {
				h.log.Errorf("Cannot preset value while creating model: '%s'.'%s'", model.ModelType.Name(), err)
				h.MarshalInternalError(rw)
				return
			}

		}

		// Handle Prechecks
		for _, pair := range model.Create.Prechecks {
			values, ok := h.GetPresetValues(pair.Scope, pair.Filter, rw)
			if !ok {
				return
			}

			if err := h.SetPresetFilterValues(pair.Filter, values...); err != nil {
				h.log.Error("Cannot preset values to the filter value. %s", err)
				h.MarshalInternalError(rw)
				return
			}

			if err := h.CheckPrecheckValues(scope, pair.Filter); err != nil {
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

		// if the model is i18n-ready control it's language field value
		if scope.UseI18n() {
			lang, ok := h.CheckValueLanguage(scope, rw)
			if !ok {
				return
			}
			h.HeaderContentLanguage(rw, lang)
		}

		// if scope.UseI18n()
		repo := h.GetRepositoryByType(model.ModelType)
		if dbErr := repo.Create(scope); dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

		rw.WriteHeader(http.StatusCreated)
		h.MarshalScope(scope, rw, req)
	}
}

// Get returns a http.HandlerFunc that gets single entity from the "model's"
// repository.
func (h *JSONAPIHandler) Get(model *ModelHandler) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[model.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}
		SetContentType(rw)
		scope, errs, err := h.Controller.BuildScopeSingle(req, reflect.New(model.ModelType).Interface())
		if err != nil {
			h.log.Error(err)
			h.MarshalInternalError(rw)
			return
		}
		if errs != nil {
			h.MarshalErrors(rw, errs...)
			return
		}

		// Preset filters
		for _, presetPair := range model.Get.Prechecks {
			values, ok := h.GetPresetValues(presetPair.Scope, presetPair.Filter, rw)
			if !ok {
				return
			}
			if err := h.SetPresetFilterValues(presetPair.Filter, values...); err != nil {
				h.log.Errorf("Error while preseting filter for model: '%s'. '%s'", model.ModelType.Name(), err)
				h.MarshalInternalError(rw)
				return
			}
			if len(presetPair.Filter.Relationships) > 0 {
				scope.RelationshipFilters = append(scope.RelationshipFilters, presetPair.Filter)
			} else {
				scope.PrimaryFilters = append(scope.PrimaryFilters, presetPair.Filter)
			}
		}

		tag, ok := h.GetLanguage(req, rw)
		if !ok {
			return
		}

		if scope.UseI18n() {
			scope.SetLanguageFilter(tag.String())
		}

		repo := h.GetRepositoryByType(model.ModelType)

		// Set NewSingleValue for the scope
		scope.NewValueSingle()
		dbErr := repo.Get(scope)
		if dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

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
func (h *JSONAPIHandler) GetRelated(root *ModelHandler) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[root.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}
		SetContentType(rw)
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

		for _, presetPair := range root.Get.Prechecks {
			values, ok := h.GetPresetValues(presetPair.Scope, presetPair.Filter, rw)
			if !ok {
				return
			}
			if err := h.SetPresetFilterValues(presetPair.Filter, values...); err != nil {
				h.log.Errorf("Error while preseting filter for model: '%s'. '%s'", root.ModelType.Name(), err)
				h.MarshalInternalError(rw)
				return
			}

			h.addPresetFilter(scope, presetPair.Filter)
		}

		tag, ok := h.GetLanguage(req, rw)
		if !ok {
			return
		}

		if scope.UseI18n() {
			scope.SetLanguageFilter(tag.String())
		}

		// Get root repository
		rootRepository := h.GetRepositoryByType(root.ModelType)
		h.log.Debugf("Getting related root for: '%s'", scope.Struct.GetCollectionType())
		scope.NewValueSingle()

		// Get the root for given id
		// Select the related field inside
		dbErr := rootRepository.Get(scope)
		if dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

		relatedScope, err := scope.GetRelatedScope()
		if err != nil {
			h.log.Errorf("Error while getting Related Scope: %v", err)
			h.MarshalInternalError(rw)
		}

		// if there is any primary filter
		h.log.Debugf("Getting related scope.")
		if relatedScope.Value != nil && len(relatedScope.PrimaryFilters) != 0 {
			h.log.Debugf("Related prim filters: %+v", relatedScope.PrimaryFilters[0])
			h.log.Debugf("Related attr filters: %+v", relatedScope.AttributeFilters)
			h.log.Debugf("Related relationship filters: %+v", relatedScope.RelationshipFilters)

			relatedRepository := h.GetRepositoryByType(relatedScope.Struct.GetType())
			if relatedScope.UseI18n() {
				relatedScope.SetLanguageFilter(tag.String())
			}
			if relatedScope.IsMany {
				h.log.Debug("The related scope isMany.")
				dbErr = relatedRepository.List(relatedScope)
			} else {
				h.log.Debug("The related scope isSingle.")

				dbErr = relatedRepository.Get(relatedScope)
			}
			if dbErr != nil {
				h.manageDBError(rw, dbErr)
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
func (h *JSONAPIHandler) GetRelationship(root *ModelHandler) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[root.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}
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

		tag, ok := h.GetLanguage(req, rw)
		if !ok {
			return
		}

		if scope.UseI18n() {
			scope.SetLanguageFilter(tag.String())
		}
		h.HeaderContentLanguage(rw, tag)

		// Preset Values
		for _, presetPair := range root.Get.Prechecks {
			values, ok := h.GetPresetValues(presetPair.Scope, presetPair.Filter, rw)
			if !ok {
				return
			}
			if err := h.SetPresetFilterValues(presetPair.Filter, values...); err != nil {
				h.log.Errorf("Error while preseting filter for model: '%s'. '%s'", root.ModelType.Name(), err)
				h.MarshalInternalError(rw)
				return
			}

			h.addPresetFilter(scope, presetPair.Filter)
		}

		// Get value from repository
		rootRepository := h.GetRepositoryByType(scope.Struct.GetType())
		scope.NewValueSingle()
		dbErr := rootRepository.Get(scope)
		if dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

		relationshipScope, err := scope.GetRelationshipScope()
		if err != nil {
			h.log.Errorf("Error while getting RelationshipScope for model: %v. %v", scope.Struct.GetType(), err)
			h.MarshalInternalError(rw)
			return
		}
		h.MarshalScope(relationshipScope, rw, req)
	}
}

// List returns a http.HandlerFunc that response with the model's entities taken
// from it's repository.
// QueryParameters:
//	- filter - filter parameter must be followed by the collection name within brackets
// 		i.e. '[collection]' and the field scoped for the filter within brackets, i.e. '[id]'
//		i.e. url: http://myapiurl.com/api/blogs?filter[blogs][id]=4
func (h *JSONAPIHandler) List(model *ModelHandler) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[model.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}
		SetContentType(rw)
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

		tag, ok := h.GetLanguage(req, rw)
		if !ok {
			return
		}
		if scope.UseI18n() {
			scope.SetLanguageFilter(tag.String())
		}

		h.HeaderContentLanguage(rw, tag)

		for _, presetPair := range model.List.Prechecks {
			values, ok := h.GetPresetValues(presetPair.Scope, presetPair.Filter, rw)
			if !ok {
				return
			}
			if err := h.SetPresetFilterValues(presetPair.Filter, values...); err != nil {
				h.log.Errorf("Error while preseting filter for model: '%s'. '%s'", model.ModelType.Name(), err)
				h.MarshalInternalError(rw)
				return
			}

			h.addPresetFilter(scope, presetPair.Filter)
		}

		repo := h.GetRepositoryByType(model.ModelType)

		scope.NewValueMany()
		dbErr := repo.List(scope)
		if dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

		if correct := h.GetIncluded(scope, rw, req, tag); !correct {
			return
		}

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
func (h *JSONAPIHandler) Patch(model *ModelHandler) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[model.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}
		// UnmarshalScope from the request body.
		SetContentType(rw)
		scope := h.UnmarshalScope(model.ModelType, rw, req)
		if scope == nil {
			return
		}

		// Set the ID for given model's scope
		err := h.Controller.GetAndSetIDFilter(req, scope)
		if err != nil {
			h.errSetIDFilter(scope, err, rw, req)
			return
		}

		if scope.UseI18n() {
			tag, ok := h.CheckValueLanguage(scope, rw)
			if !ok {
				return
			}
			h.HeaderContentLanguage(rw, tag)
		}

		for _, presetPair := range model.Patch.Presets {
			values, ok := h.GetPresetValues(presetPair.Scope, presetPair.Filter, rw)
			if !ok {
				return
			}

			if err := h.PresetScopeValue(scope, presetPair.Filter, values...); err != nil {
				h.log.Errorf("Cannot preset value while creating model: '%s'.'%s'", model.ModelType.Name(), err)
				h.MarshalInternalError(rw)
				return
			}
		}

		// Precheck filters
		for _, presetPair := range model.Patch.Prechecks {
			values, ok := h.GetPresetValues(presetPair.Scope, presetPair.Filter, rw)
			if !ok {
				return
			}
			if err := h.SetPresetFilterValues(presetPair.Filter, values...); err != nil {
				h.log.Errorf("Error while preseting filter for model: '%s'. '%s'", model.ModelType.Name(), err)
				h.MarshalInternalError(rw)
				return
			}
			h.addPresetFilter(scope, presetPair.Filter)
		}

		// Get the Repository for given model
		repo := h.GetRepositoryByType(model.ModelType)

		// Use Patch Method on given model's Repository for given scope.
		if dbErr := repo.Patch(scope); dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}
		rw.WriteHeader(http.StatusNoContent)
		return
	}
}

func (h *JSONAPIHandler) Delete(model *ModelHandler) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[model.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}
		// Create a scope for given delete handler
		scope, err := h.Controller.NewScope(reflect.New(model.ModelType).Interface())
		if err != nil {
			h.log.Errorf("Error while creating scope: '%v' for model: '%v'", err, reflect.TypeOf(model))
			h.MarshalInternalError(rw)
			return
		}

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

		// Preset filters
		for _, presetPair := range model.Delete.Prechecks {
			values, ok := h.GetPresetValues(presetPair.Scope, presetPair.Filter, rw)
			if !ok {
				return
			}
			if err := h.SetPresetFilterValues(presetPair.Filter, values...); err != nil {
				h.log.Errorf("Error while preseting filter for model: '%s'. '%s'", model.ModelType.Name(), err)
				h.MarshalInternalError(rw)
				return
			}
			if len(presetPair.Filter.Relationships) > 0 {
				scope.RelationshipFilters = append(scope.RelationshipFilters, presetPair.Filter)
			} else {
				scope.PrimaryFilters = append(scope.PrimaryFilters, presetPair.Filter)
			}
		}

		repo := h.GetRepositoryByType(model.ModelType)

		if dbErr := repo.Delete(scope); dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

		rw.WriteHeader(http.StatusNoContent)
	}
}
