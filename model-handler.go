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
	endpoints []*Endpoint,
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
		switch endpoint.Type {
		case Create:
			m.Create = endpoint
		case Get:
			m.Get = endpoint
		case List:
			m.List = endpoint
		case Patch:
			m.Patch = endpoint
		case Delete:
			m.Delete = endpoint
		default:
			err = fmt.Errorf("Provided invalid endpoint type for model: %s", m.ModelType.Name())
			return
		}
	}
}

// AddPresetScope adds preset scope to provided endpoint. If the endpoint was not set an error
// occurs
func (m *ModelHandler) AddPresetScope(presetScope *jsonapi.Scope, endpoint EndpointType) error {
	nilEndpoint := func(eName string) error {
		return fmt.Errorf("Adding preset scope on the nil '%s' Endpoint on model: '%s'", eName, m.ModelType.Name())
	}

	switch endpoint {
	case Create:
		if m.Create == nil {
			return nilEndpoint("Create")
		}
		m.Create.PresetScope = presetScope
	case Get:
		if m.Get == nil {
			return nilEndpoint("Get")
		}
		m.Get.PresetScope = presetScope
	case List:
		if m.List == nil {
			return nilEndpoint("List")
		}
		m.List.PresetScope = presetScope
	case Patch:
		if m.Patch == nil {
			return nilEndpoint("Patch")
		}
		m.Patch.PresetScope = presetScope
	case Delete:
		if m.Delete == nil {
			return nilEndpoint("Delete")
		}
		m.Delete.PresetScope = presetScope
	default:
		return errors.New("Endpoint not specified.")
	}
	return nil
}

type Endpoint struct {
	Type        EndpointType
	PresetScope *jsonapi.Scope
}
