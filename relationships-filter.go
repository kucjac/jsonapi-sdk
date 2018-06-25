package jsonapisdk

import (
	"github.com/kucjac/jsonapi"
	"net/http"
	"reflect"
)

func (h *JSONAPIHandler) GetRelationshipFilters(scope *jsonapi.Scope, req *http.Request, rw http.ResponseWriter) error {

	h.log.Debug("-------Getting Relationship Filters--------")
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
			for _, precheck := range relModel.List.PrecheckPairs {
				precheckScope, precheckField := precheck.GetPair()
				if precheck.Key != nil {
					if !h.getPrecheckFilter(precheck.Key, precheckScope, req, relModel) {
						continue
					}
				}
				values, err := h.GetPresetValues(precheckScope, rw)
				if err != nil {
					if hErr := err.(*HandlerError); hErr != nil {
						return hErr

					} else {
						return err
					}
				}

				if err := h.SetPresetFilterValues(precheckField, values...); err != nil {
					hErr := newHandlerError(ErrValuePreset, err.Error())
					hErr.Field = precheckField.StructField
					return hErr
				}

				if err := relationshipScope.AddFilterField(precheckField); err != nil {
					hErr := newHandlerError(ErrValuePreset, err.Error())
					hErr.Field = precheckField.StructField
					return hErr
				}
			}
		}

		var (
			attrFilter bool
			primFilter bool
		)

		// Get relationship scope filters
		for _, subFieldFilter := range relFilter.Relationships {
			switch subFieldFilter.GetFieldKind() {
			case jsonapi.Primary:
				relationshipScope.PrimaryFilters = append(relationshipScope.PrimaryFilters, subFieldFilter)
				primFilter = true
			case jsonapi.Attribute:
				relationshipScope.AttributeFilters = append(relationshipScope.AttributeFilters, subFieldFilter)
				attrFilter = true

			default:
				h.log.Warningf("The subfield of the filter cannot be of relationship filter type. Model: '%s',", scope.Struct.GetType().Name(), req.URL.Path)
			}
		}

		if primFilter && !attrFilter {
			continue
		}

		// Get the relationship scope
		relationshipScope.NewValueMany()

		if errObj := h.HookBeforeReader(relationshipScope); errObj != nil {
			return errObj
		}

		dbErr := h.GetRepositoryByType(relationshipScope.Struct.GetType()).List(relationshipScope)
		if dbErr != nil {
			return dbErr
		}

		if errObj := h.HookAfterReader(relationshipScope); errObj != nil {
			return errObj
		}

		values, err := relationshipScope.GetPrimaryFieldValues()
		if err != nil {
			h.log.Debugf("GetPrimaryFieldValues error within GetRelationship function. %v", err)
			hErr := newHandlerError(ErrBadValues, err.Error())
			hErr.Model = relFilter.GetRelatedModelStruct()
			return hErr
		}

		if len(values) == 0 {
			hErr := newHandlerError(ErrNoValues, "")
			hErr.Model = relFilter.GetRelatedModelStruct()
			hErr.Scope = relationshipScope
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
