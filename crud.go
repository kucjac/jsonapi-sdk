package jsonapisdk

import (
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
func (h *JSONAPIHandler) Create(model interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		SetContentType(rw)
		scope := h.UnmarshalScope(model, rw, req)
		if scope == nil {
			return
		}

		// if the model is i18n-ready control it's language field value
		if scope.UseI18n() {
			lang, ok := h.CheckValueLanguage(scope, rw)
			if !ok {
				return
			}
			h.HeaderContentLanguage(rw, lang)
		}

		repo := h.GetModelsRepository(model)

		// if scope.UseI18n()
		if dbErr := repo.Create(scope); dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}
		h.MarshalScope(scope, rw, req)
		rw.WriteHeader(http.StatusCreated)
	}
}

// Get returns a http.HandlerFunc that gets single entity from the "model's"
// repository.
func (h *JSONAPIHandler) Get(model interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		SetContentType(rw)
		scope, errs, err := h.Controller.BuildScopeSingle(req, model)
		if err != nil {
			h.log.Error(err)
			h.MarshalInternalError(rw)
			return
		}
		if errs != nil {
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

		repo := h.GetModelsRepository(model)

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
func (h *JSONAPIHandler) GetRelated(root interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		SetContentType(rw)
		scope, errs, err := h.Controller.BuildScopeRelated(req, root)
		if err != nil {
			h.log.Errorf("An internal error occurred while building related scope for model: '%v'. %v", reflect.TypeOf(root), err)
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

		// Get root repository
		rootRepository := h.GetModelsRepository(root)

		scope.NewValueSingle()
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
		if relatedScope.Value != nil && len(relatedScope.PrimaryFilters) != 0 {
			h.log.Debug(relatedScope.PrimaryFilters)
			relatedRepository := h.GetModelRepositoryByType(relatedScope.Struct.GetType())
			if relatedScope.UseI18n() {
				relatedScope.SetLanguageFilter(tag.String())
			}
			if relatedScope.IsMany {
				dbErr = relatedRepository.List(relatedScope)
			} else {
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
func (h *JSONAPIHandler) GetRelationship(root interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		scope, errs, err := h.Controller.BuildScopeRelationship(req, root)
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

		rootRepository := h.GetModelRepositoryByType(scope.Struct.GetType())
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
func (h *JSONAPIHandler) List(model interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		SetContentType(rw)
		scope, errs, err := h.Controller.BuildScopeList(req, model)
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
		repo := h.GetModelsRepository(model)

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

func (h *JSONAPIHandler) Patch(model interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		// UnmarshalScope from the request body.
		SetContentType(rw)
		scope := h.UnmarshalScope(model, rw, req)
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

		// Get the Repositoriesitory for given model
		repo := h.GetModelsRepository(model)

		// Use Patch Method on given model's Repositoriesitory for given scope.
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
		scope, err := h.Controller.NewScope(model)
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

		repo := h.GetModelsRepository(model)

		if dbErr := repo.Delete(scope); dbErr != nil {
			h.manageDBError(rw, dbErr)
			return
		}

		rw.WriteHeader(http.StatusNoContent)
	}
}
