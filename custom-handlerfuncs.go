package jsonapisdk

import (
	"github.com/kucjac/jsonapi"
	"net/http"
)

// GetNoID is a handler func that get's the model object from the preset filter or preset pair
// function values
func (h *JSONAPIHandler) GetNoID(model *ModelHandler, endpoint *Endpoint) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if _, ok := h.ModelHandlers[model.ModelType]; !ok {
			h.MarshalInternalError(rw)
			return
		}
		SetContentType(rw)

		/**

		  GET-NOID: PRESET PAIR

		*/
		var id interface{}
		for _, presetPair := range endpoint.PresetPairs {
			presetScope, presetFilter := presetPair.GetPair()
			if presetPair.Key != nil {
				if !h.getPrecheckFilter(presetPair.Key, presetScope, req, model) {
					continue
				}
			}

			values, err := h.GetPresetValues(presetScope, rw)
			if err != nil {
				if hErr := err.(*HandlerError); hErr != nil {
				if hErr.Code == ErrNoValues {					
					errObj := jsonapi.ErrResourceNotFound.Copy()
					h.MarshalErrors(rw, errObj)
					return
				}
				if !h.handleHandlerError(hErr, rw) {
					return
				}
				h.log.Errorf("Unknown  error while presetting id from presetpair: %v", err)
				h.MarshalInternalError(rw)
				return
			}

			if len(values) == 0 {
				h.MarshalErrors(rw, jsonapi.ErrResourceNotFound.Copy())
				return
			}
			id = values[0]
		}

		if id == nil {
			for _, presetFilter := range endpoint.PresetFilters {
				
			}
		}

		/**

		GET: BUILD SCOPE

		*/
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
	}
}
