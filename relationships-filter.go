package jsonapisdk

import (
	"github.com/kucjac/jsonapi"
	"net/http"
	"reflect"
)

func (h *JSONAPIHandler) GetRelationshipFilters(scope *jsonapi.Scope, req *http.Request, rw http.ResponseWriter) error {
	// every relationship filter is for different field
	// replace the filter with the preset values of id field
	// so that the repository should not handle the relationship filter
	for i, relFilter := range scope.RelationshipFilters {

		// Every relationship filter may contain multiple subfilters
		relationshipScope, err := h.Controller.NewScope(reflect.New(relFilter.GetRelatedModelType()).Interface())
		if err != nil {
			// internal
			// model not precomputed
			hErr := newHandlerError(ErrNoModel, "Cannot get new scope.")
			hErr.Model = relFilter.GetRelatedModelStruct()
			return hErr
		}

		relationshipScope.Fieldset = nil

		// Get PresetFilters for the relationship model type
		//	i.e. materials -> storage
		//	storage should have {preset=panel-info.supplier} {filter[panel-info][id]=some-id}
		//

		relModel, ok := h.ModelHandlers[relFilter.GetRelatedModelType()]
		if !ok {
			hErr := newHandlerError(ErrNoModel, "Cannot get model handler")
			hErr.Model = relFilter.GetRelatedModelStruct()
			return hErr
		}
		if relModel.List != nil {
			for _, precheck := range relModel.List.Prechecks {
				values, ok := h.GetPresetValues(precheck.Scope, precheck.Filter, rw)
				if !ok {
					hErr := newHandlerError(ErrAlreadyWritten, "")
					return hErr
				}

				if err := h.SetPresetFilterValues(precheck.Filter, values...); err != nil {
					hErr := newHandlerError(ErrValuePreset, err.Error())
					hErr.Field = precheck.Filter.StructField
					return hErr
				}

				h.addPresetFilter(relationshipScope, precheck.Filter)
			}
		}

		// Get relationship scope filters
		for _, subFieldFilter := range relFilter.Relationships {
			switch subFieldFilter.GetFieldKind() {
			case jsonapi.Primary:
				relationshipScope.PrimaryFilters = append(relationshipScope.PrimaryFilters, subFieldFilter)
			case jsonapi.Attribute:
				relationshipScope.AttributeFilters = append(relationshipScope.AttributeFilters, subFieldFilter)
			default:
				h.log.Warningf("The subfield of the filter cannot be of relationship filter type. Model: '%s', Path: '%s'", scope.Struct.GetType().Name(), req.URL.Path)
			}
		}

		// Get the relationship scope
		relationshipScope.NewValueMany()
		dbErr := h.GetRepositoryByType(relationshipScope.Struct.GetType()).List(relationshipScope)
		if dbErr != nil {
			return dbErr
		}

		values, err := relationshipScope.GetPrimaryFieldValues()
		if err != nil {
			hErr := newHandlerError(ErrNoValues, err.Error())
			hErr.Model = relFilter.GetRelatedModelStruct()
			return hErr
		}

		subField := &jsonapi.FilterField{
			StructField: relFilter.GetRelatedModelStruct().GetPrimaryField(),
			Values:      []*jsonapi.FilterValues{{Operator: jsonapi.OpIn, Values: values}},
		}
		relationFilter := &jsonapi.FilterField{StructField: relFilter.StructField, Relationships: []*jsonapi.FilterField{subField}}

		scope.RelationshipFilters[i] = relationFilter
	}
	return nil
}
