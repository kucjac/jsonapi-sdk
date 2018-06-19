package jsonapisdk

import (
	"errors"
	"fmt"
	"github.com/kucjac/jsonapi"
	"net/http"
	"reflect"
)

var IErrInvalidModelEndpoint = errors.New("Invalid model endpoint")

type MiddlewareFunc func(next http.HandlerFunc) http.HandlerFunc

// ModelHandler defines how the
type ModelHandler struct {
	ModelType reflect.Type

	// Endpoints contains preset information about the provided model.
	Create *Endpoint
	Get    *Endpoint
	List   *Endpoint
	Patch  *Endpoint
	Delete *Endpoint

	// Repository defines the repository for the provided model
	Repository Repository
}

type ModelPresetGetter interface {
	GetPresetPair(endpoint EndpointType, controller *jsonapi.Controller) *jsonapi.PresetPair
}

type ModelPrecheckGetter interface {
	GetPrecheckPair(endpoint EndpointType, controller *jsonapi.Controller) *jsonapi.PresetPair
}

// NewModelHandler creates new model handler for given model, with provided repository and with
// support for provided endpoints.
// Returns an error if provided model is not a struct or a ptr to struct or if the endpoint is  of
// unknown type.
func NewModelHandler(
	model interface{},
	repository Repository,
	endpoints []EndpointType,
) (m *ModelHandler, err error) {
	m = new(ModelHandler)

	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		err = errors.New("Invalid model provided. Model must be struct or a pointer to struct.")
		return
	}

	m.ModelType = t
	m.Repository = repository
	for _, endpoint := range endpoints {
		switch endpoint {
		case Create:
			m.Create = &Endpoint{Type: endpoint}
		case Get:
			m.Get = &Endpoint{Type: endpoint}
		case List:
			m.List = &Endpoint{Type: endpoint}
		case Patch:
			m.Patch = &Endpoint{Type: endpoint}
		case Delete:
			m.Delete = &Endpoint{Type: endpoint}
		default:
			err = fmt.Errorf("Provided invalid endpoint type for model: %s", m.ModelType.Name())
			return
		}
	}
	return
}

// AddPresetScope adds preset scope to provided endpoint.
// If the endpoint was not set or is unknown the function returns error.
func (m *ModelHandler) AddPresetPair(
	presetPair *jsonapi.PresetPair,
	endpoint EndpointType,
) error {
	return m.addPresetPair(presetPair, endpoint, false)
}

// AddPrecheckPair adds the precheck pair to the given model on provided endpoint.
// If the endpoint was not set or is unknown the function returns error.
func (m *ModelHandler) AddPrecheckPair(
	precheckPair *jsonapi.PresetPair,
	endpoint EndpointType,
) error {
	return m.addPresetPair(precheckPair, endpoint, true)
}

func (m *ModelHandler) AddPresetFilter(
	fieldName string,
	endpointTypes []EndpointType,
	operator jsonapi.FilterOperator,
	values ...interface{},
) error {
	return nil
}

func (m *ModelHandler) AddPresetSort(
	fieldName string,
	endpointTypes []EndpointType,
	order jsonapi.Order,
) error {
	return nil
}

func (m *ModelHandler) AddOffsetPresetPaginate(
	limit, offset int,
	endpointTypes []EndpointType,
) error {
	return nil
}

// AddEndpoint adds the endpoint to the provided model handler.
// If the endpoint is of unknown type or the handler already contains given endpoint
// an error would be returned.
func (m *ModelHandler) AddEndpoint(endpoint *Endpoint) error {
	return m.changeEndpoint(endpoint, false)
}

// AddMiddlewareFunctions adds the middleware functions for given endpoint
func (m *ModelHandler) AddMiddlewareFunctions(endpoint EndpointType, middlewares ...MiddlewareFunc) error {
	var modelEndpoint *Endpoint
	switch endpoint {
	case Create:
		modelEndpoint = m.Create
	case Get:
		modelEndpoint = m.Get
	case List:
		modelEndpoint = m.List
	case Patch:
		modelEndpoint = m.Patch
	case Delete:
		modelEndpoint = m.Delete
	}

	if modelEndpoint == nil {
		err := fmt.Errorf("Invalid endpoint provided: %v", endpoint)
		return err
	}

	modelEndpoint.Middlewares = append(modelEndpoint.Middlewares, middlewares...)
	return nil
}

// ReplaceEndpoint replaces the endpoint for the provided model handler.
// If the endpoint is of unknown type the function returns an error.
func (m *ModelHandler) ReplaceEndpoint(endpoint *Endpoint) error {
	return m.changeEndpoint(endpoint, true)
}

func (m *ModelHandler) addPresetPair(
	presetPair *jsonapi.PresetPair,
	endpoint EndpointType,
	check bool,
) error {
	nilEndpoint := func(eName string) error {
		return fmt.Errorf("Adding preset scope on the nil '%s' Endpoint on model: '%s'", eName, m.ModelType.Name())
	}

	switch endpoint {
	case Create:
		if m.Create == nil {
			return nilEndpoint("Create")
		}
		if check {
			m.Create.Prechecks = append(m.Create.Prechecks, presetPair)
		} else {
			m.Create.Presets = append(m.Create.Presets, presetPair)
		}

	case Get:
		if m.Get == nil {
			return nilEndpoint("Get")
		}
		if check {
			m.Get.Prechecks = append(m.Get.Prechecks, presetPair)
		} else {
			m.Get.Presets = append(m.Get.Presets, presetPair)
		}
	case List:
		if m.List == nil {
			return nilEndpoint("List")
		}

		if check {
			m.List.Prechecks = append(m.List.Prechecks, presetPair)
		} else {
			m.List.Presets = append(m.List.Presets, presetPair)
		}

	case Patch:
		if m.Patch == nil {
			return nilEndpoint("Patch")
		}

		if check {
			m.Patch.Prechecks = append(m.Patch.Prechecks, presetPair)
		} else {
			m.Patch.Presets = append(m.Patch.Presets, presetPair)
		}

	case Delete:
		if m.Delete == nil {
			return nilEndpoint("Delete")
		}

		if check {
			m.Delete.Prechecks = append(m.Delete.Prechecks, presetPair)
		} else {
			m.Delete.Presets = append(m.Delete.Presets, presetPair)
		}

	default:
		return errors.New("Endpoint not specified.")
	}
	return nil
}

func (m *ModelHandler) changeEndpoint(endpoint *Endpoint, replace bool) error {
	var modelEndpoint *Endpoint
	switch endpoint.Type {
	case Create:
		modelEndpoint = m.Create
	case Get:
		modelEndpoint = m.Get
	case List:
		modelEndpoint = m.List
	case Patch:
		modelEndpoint = m.Patch
	case Delete:
		modelEndpoint = m.Delete
	default:
		return IErrInvalidModelEndpoint
	}

	if modelEndpoint != nil {
		return fmt.Errorf("Endpoint: '%s' already set for model: '%s'.", modelEndpoint.Type, m.ModelType.String())
	}

	*modelEndpoint = *endpoint
	return nil
}

type Endpoint struct {
	Type EndpointType

	Middlewares []MiddlewareFunc

	// Precheck
	Prechecks []*jsonapi.PresetPair

	// Preset
	Presets []*jsonapi.PresetPair

	// Preset default Filters
	PresetFilters []*jsonapi.FilterField

	// Preset default sorting
	PresetSort []*jsonapi.SortField

	// Preset default limit offset
	PresetPaginate *jsonapi.Pagination
}

func (e *Endpoint) String() string {
	return e.Type.String()
}

/**

JSONAPIHandler Methods with ModelHandler

*/

// AddModelsPresetPair gets the model handler from the JSONAPIHandler, and adds the presetpair
// to the specific endpoint for this model.
// Returns error if the model is not present within JSONAPIHandler or if the model does not
// support given endpoint.
func (h *JSONAPIHandler) AddModelsPresetPair(
	model interface{},
	presetPair *jsonapi.PresetPair,
	endpoint EndpointType,
) error {
	handler, err := h.getModelHandler(model)
	if err != nil {
		return err
	}

	if err := handler.AddPresetPair(presetPair, endpoint); err != nil {
		return err
	}
	return nil
}

// AddModelsPrecheckPair gets the model handler from the JSONAPIHandler, and adds the precheckPair
// to the specific endpoint for this model.
// Returns error if the model is not present within JSONAPIHandler or if the model does not
// support given endpoint.
func (h *JSONAPIHandler) AddModelsPrecheckPair(
	model interface{},
	precheckPair *jsonapi.PresetPair,
	endpoint EndpointType,
) error {
	handler, err := h.getModelHandler(model)
	if err != nil {
		return err
	}

	if err := handler.AddPresetPair(precheckPair, endpoint); err != nil {
		return err
	}
	return nil
}

// AddModelsEndpoint adds the endpoint to the provided model.
// If the model is not set within given handler, an endpoint is already occupied or is of unknown
// type the function returns error.
func (h *JSONAPIHandler) AddModelsEndpoint(model interface{}, endpoint *Endpoint) error {
	handler, err := h.getModelHandler(model)
	if err != nil {
		return err
	}
	if err := handler.AddEndpoint(endpoint); err != nil {
		return err
	}
	return nil
}

// ReplaceModelsEndpoint replaces an endpoint for provided model.
// If the model is not set within JSONAPIHandler or an endpoint is of unknown type the function
// returns an error.
func (h *JSONAPIHandler) ReplaceModelsEndpoint(model interface{}, endpoint *Endpoint) error {
	handler, err := h.getModelHandler(model)
	if err != nil {
		return err
	}
	if err := handler.ReplaceEndpoint(endpoint); err != nil {
		return err
	}
	return nil
}

// GetModelHandler gets the model handler that matches the provided model type.
// If no handler is found within JSONAPIHandler the function returns an error.
func (h *JSONAPIHandler) GetModelHandler(model interface{}) (mHandler *ModelHandler, err error) {
	return h.getModelHandler(model)
}

func (h *JSONAPIHandler) getModelHandler(model interface{}) (mHandler *ModelHandler, err error) {
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}
	var ok bool
	mHandler, ok = h.ModelHandlers[modelType]
	if !ok {
		err = IErrModelHandlerNotFound
		return
	}
	return

}
