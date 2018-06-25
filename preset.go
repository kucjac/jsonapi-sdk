package jsonapisdk

import (
	"errors"
	"github.com/kucjac/jsonapi"
	"net/http"
	"reflect"
	"runtime/debug"
)

func (h *JSONAPIHandler) AddPrecheckFilters(
	scope *jsonapi.Scope,
	req *http.Request,
	rw http.ResponseWriter,
	filters ...*jsonapi.PresetFilter,
) (ok bool) {
	for _, filter := range filters {
		h.log.Debugf("Adding precheck filter: %s", filter.GetFieldName())

		value := req.Context().Value(filter.Key)
		if value == nil {
			continue
		}

		if err := h.SetPresetFilterValues(filter.FilterField, value); err != nil {
			h.log.Errorf("Error while setting values for filter field. Model: %v, Filterfield: %v. Error: %v", scope.Struct.GetType().Name(), filter.GetFieldName(), err)
			h.MarshalInternalError(rw)
			return false
		}
		if err := scope.AddFilterField(filter.FilterField); err != nil {
			h.log.Errorf("Cannot add filter field to root scope in get related field. %v", err)
			h.MarshalInternalError(rw)
			return false
		}
	}
	return true
}

func (h *JSONAPIHandler) AddPrecheckPairFilters(
	scope *jsonapi.Scope,
	model *ModelHandler,
	endpoint *Endpoint,
	req *http.Request,
	rw http.ResponseWriter,
	pairs ...*jsonapi.PresetPair,
) (ok bool) {
	for _, presetPair := range pairs {
		presetScope, presetField := presetPair.GetPair()
		if presetPair.Key != nil {
			if !h.getPrecheckFilter(presetPair.Key, presetScope, req, model) {
				continue
			}
		}
		values, err := h.GetPresetValues(presetScope, presetField, rw)
		if err != nil {
			if hErr := err.(*HandlerError); hErr != nil {
				if hErr.Code == ErrNoValues {
					if endpoint.Type == List {
						h.MarshalScope(scope, rw, req)
						return
					}
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
			// if handleHandlerError has warning
			continue
		}
		if err := h.SetPresetFilterValues(presetField, values...); err != nil {
			h.log.Errorf("Error while preseting filter for model: '%s'. '%s'", model.ModelType.Name(), err)
			h.MarshalInternalError(rw)
			return
		}

		h.addPresetFilter(scope, presetField)
	}
	return true
}

// GetPresetValues gets the values from the presetScope
func (h *JSONAPIHandler) GetPresetValues(
	presetScope *jsonapi.Scope,
	filter *jsonapi.FilterField,
	rw http.ResponseWriter,
) (values []interface{}, err error) {
	h.log.Debug("------Getting Preset Values-------")
	h.log.Debug("Preset Fieldset:")
	for field := range presetScope.Fieldset {
		h.log.Debug(field)
	}

	repo := h.GetRepositoryByType(presetScope.Struct.GetType())

	presetScope.NewValueMany()

	if errObj := h.HookBeforeReader(presetScope); errObj != nil {
		h.MarshalErrors(rw)
		err = newHandlerError(ErrAlreadyWritten, errObj.Error())
		return
	}

	dbErr := repo.List(presetScope)
	if dbErr != nil {
		h.manageDBError(rw, dbErr)
		err = newHandlerError(ErrAlreadyWritten, dbErr.Message)
		return
	}
	v := reflect.ValueOf(presetScope.Value)
	for i := 0; i < v.Len(); i++ {
		h.log.Debugf("Value of presetscope: %+v at Index: %v", v.Index(i).Interface(), i)
	}

	if errObj := h.HookAfterReader(presetScope); errObj != nil {
		h.MarshalErrors(rw)
		err = newHandlerError(ErrAlreadyWritten, errObj.Error())
		return
	}

	scopeVal := reflect.ValueOf(presetScope.Value)
	if scopeVal.Len() == 0 {
		hErr := newHandlerError(ErrNoValues, "Provided resource does not exists.")
		err = hErr
		return
	}

	// set the primary values for the collection scope
	if err = presetScope.SetCollectionValues(); err != nil {
		hErr := newHandlerError(ErrInternal, err.Error())
		hErr.Scope = presetScope
		err = hErr
		return
	}

	for presetScope.NextIncludedField() {
		var field *jsonapi.IncludeField
		field, err = presetScope.CurrentIncludedField()
		if err != nil {
			hErr := newHandlerError(ErrInternal, err.Error())
			hErr.Scope = presetScope
			err = hErr
			return
		}

		var missing []interface{}
		missing, err = field.GetMissingPrimaries()
		if err != nil {
			hErr := newHandlerError(ErrInternal, err.Error())
			hErr.Field = field.StructField
			err = hErr
			return
		}
		if len(missing) == 0 {
			h.log.Debugf("Missing error")
			hErr := newHandlerError(ErrNoValues, "")
			err = hErr
			return
		}

		// Add the missing id filters
		field.Scope.SetIDFilters(missing...)

		if len(field.Scope.IncludedFields) != 0 {
			values, err = h.GetPresetValues(field.Scope, filter, rw)
			if err != nil {
				return
			}
			return
		}
		return missing, nil
	}

	return
}

// PresetScopeValue presets provided values for given scope.
// The fieldFilter points where the value should be set within given scope.
// The scope value should not be nil
func (h *JSONAPIHandler) PresetScopeValue(
	scope *jsonapi.Scope,
	fieldFilter *jsonapi.FilterField,
	values ...interface{},
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			h.log.Errorf("Panic within preset scope value. %v. %v", r, string(debug.Stack()))
			err = IErrPresetInvalidScope
		}
	}()

	if scope.Value == nil {
		return IErrScopeNoValue
	}

	if len(values) == 0 {
		return IErrPresetNoValues
	}

	v := reflect.ValueOf(scope.Value)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		if v.Kind() != reflect.Struct {
			return IErrPresetInvalidScope
		}
	}

	fIndex := fieldFilter.GetFieldIndex()
	field := v.Field(fIndex)

	if len(fieldFilter.Relationships) != 0 {
		switch fieldFilter.GetFieldKind() {

		case jsonapi.RelationshipSingle:
			relIndex := fieldFilter.Relationships[0].GetFieldIndex()
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			relatedField := field.Elem().Field(relIndex)
			switch relatedField.Kind() {
			case reflect.Slice:
				refValues := reflect.ValueOf(values)
				relatedField = reflect.AppendSlice(relatedField, refValues)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				relatedField.Set(reflect.ValueOf(values[0]))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				relatedField.Set(reflect.ValueOf(values[0]))
			case reflect.Float32, reflect.Float64:
				relatedField.Set(reflect.ValueOf(values[0]))
			case reflect.String:
				relatedField.Set(reflect.ValueOf(values[0]))
			case reflect.Struct:
				relatedField.Set(reflect.ValueOf(values[0]))
			case reflect.Ptr:
				relatedField.Set(reflect.ValueOf(values[0]))
			}

			if len(values) > 1 {
				h.log.Errorf("Provided values length is greatern than 1 but the field is not of slice type. Presetting only the first value. Field: '%s'.", field.String())
			}

		case jsonapi.RelationshipMultiple:
			fieldElemType := fieldFilter.GetRelatedModelType()
			relatedIndex := fieldFilter.Relationships[0].GetFieldIndex()
			for _, value := range values {
				refValue := reflect.ValueOf(value)
				fieldElem := reflect.New(fieldElemType)
				relatedField := fieldElem.Field(relatedIndex)
				switch relatedField.Kind() {
				case reflect.Slice:
					relatedField = reflect.Append(relatedField, refValue)
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					relatedField.Set(refValue)
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					relatedField.Set(refValue)
				case reflect.Float32, reflect.Float64:
					relatedField.Set(refValue)
				case reflect.String:
					relatedField.Set(refValue)
				case reflect.Struct:
					relatedField.Set(refValue)
				case reflect.Ptr:
					relatedField.Set(refValue)
				}

				field = reflect.Append(field, relatedField)
			}
		default:
			h.log.Error("Relationship filter is of invalid type. Model: '%s'. Field: '%s'", scope.Struct.GetType().Name(), field.String())
			return IErrPresetInvalidScope
		}

	} else {

		switch field.Kind() {
		case reflect.Slice:
			refValues := reflect.ValueOf(values)
			field = reflect.AppendSlice(field, refValues)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.Set(reflect.ValueOf(values[0]))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.Set(reflect.ValueOf(values[0]))
		case reflect.Float32, reflect.Float64:
			field.Set(reflect.ValueOf(values[0]))
		case reflect.String:
			field.Set(reflect.ValueOf(values[0]))
		case reflect.Struct:
			field.Set(reflect.ValueOf(values[0]))
		case reflect.Ptr:
			field.Set(reflect.ValueOf(values[0]))
		}

		if len(values) > 1 {
			h.log.Warningf("Provided values length is greatern than 1 but the field is not of slice type. Presetting only the first value. Field: '%s'.", field.String())
		}
	}
	return nil
}

func (h *JSONAPIHandler) SetPresetFilters(
	scope *jsonapi.Scope,
	model *ModelHandler,
	req *http.Request,
	rw http.ResponseWriter,
	filters ...*jsonapi.PresetFilter,
) (ok bool) {
	for _, filter := range filters {
		if value := req.Context().Value(filter.Key); value != nil {
			if err := h.SetPresetFilterValues(filter.FilterField, value); err != nil {
				h.log.Errorf("Cannot set preset filter values. Model: %v, Filterfield: %v, Value: %v, Path: %v. Error: %v", model.ModelType.Name(), filter.GetFieldName(), value, req.URL.Path, err)
				h.MarshalInternalError(rw)
				return
			}
		}
		if err := scope.AddFilterField(filter.FilterField); err != nil {
			h.log.Errorf("Cannot add filter field. Path: %v, Error: %v", req.URL.Path, err)
			h.MarshalInternalError(rw)
			return
		}
	}
	return true
}

// SetPresetValues sets provided values for given filterfield.
// If the filterfield does not contain values or subfield values the function returns error.
func (h *JSONAPIHandler) SetPresetFilterValues(
	filter *jsonapi.FilterField,
	values ...interface{},
) error {
	if len(filter.Values) != 0 {
		fv := filter.Values[0]
		(*fv).Values = values
		return nil
	} else if len(filter.Relationships) != 0 {
		if len(filter.Relationships[0].Values) != 0 {
			fv := filter.Relationships[0].Values[0]
			(*fv).Values = values
			return nil
		}
	}
	return errors.New("Invalid preset filter.")
}

/**

PRIVATE

*/

func (h *JSONAPIHandler) getPresetFilter(
	key interface{},
	presetScope *jsonapi.Scope,
	req *http.Request,
	model *ModelHandler,
) bool {
	return h.getPrecheckFilter(key, presetScope, req, model)
}

func (h *JSONAPIHandler) getPrecheckFilter(
	key interface{},
	precheckScope *jsonapi.Scope,
	req *http.Request,
	model *ModelHandler,
) (exists bool) {
	precheckValue := req.Context().Value(key)
	if precheckValue == nil {
		h.log.Warningf("Precheck value for model: %v is not set at endpoint CREATE", model.ModelType.Name())
		return
	}

	precheckFilter, ok := precheckValue.(*jsonapi.FilterField)
	if !ok {
		presetFilter, ok := precheckValue.(*jsonapi.PresetFilter)
		if !ok {
			h.log.Warningf("PrecheckValue is not a jsonapi.FilterField. Model: %v, endpoint: CREATE", model.ModelType.Name())
			return
		}
		precheckFilter = presetFilter.FilterField
	}
	if !h.addPresetFilterToPresetScope(precheckScope, precheckFilter) {
		return
	}
	return true
}
