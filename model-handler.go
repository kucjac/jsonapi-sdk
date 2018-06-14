package jsonapisdk

import (
	"errors"
	"fmt"
	"github.com/kucjac/jsonapi"
	"golang.org/x/text/language"
	"reflect"
)

// ModelHandler defines how the
type ModelHandler struct {
	ModelType reflect.Type

	Create *Endpoint
	Get    *Endpoint
	List   *Endpoint
	Patch  *Endpoint
	Delete *Endpoint

	Repository Repository

	Languages []language.Tag
}

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

type Endpoint struct {
	Type EndpointType

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
