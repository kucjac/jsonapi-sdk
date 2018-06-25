package jsonapisdk

import (
	"github.com/kucjac/jsonapi"
	"net/http"
)

type Endpoint struct {
	// Type is the endpoint type
	Type EndpointType

	// PrecheckPairs are the pairs of jsonapi.Scope and jsonapi.Filter
	// The scope deines the model from where the preset values should be taken
	// The second defines the filter field for the target model's scope that would be filled with
	// the values from the precheckpair scope
	PrecheckPairs []*jsonapi.PresetPair

	// PresetPairs are the paris of jsonapi.Scope and jsonapiFilter
	// The scope defines the model from where the preset values should be taken. It should not be
	// the same as target model.
	// The second parameter defines the target model's field and it subfield where the value
	// should be preset.
	PresetPairs []*jsonapi.PresetPair

	// PresetFilters are the filters for the target model's scope
	// They should be filled with values taken from context with key "jsonapi.PresetFilterValue"
	// When the values are taken the target model's scope would save the value for the relation
	// provided in the filterfield.
	PresetFilters []*jsonapi.PresetFilter

	// PrecheckFilters are the filters for the target model's scope
	// They should be filled with values taken from context with key "jsonapi.PrecheckPairFilterValue"
	// When the values are taken and saved into the precheck filter, the filter is added into the
	// target model's scope.
	PrecheckFilters []*jsonapi.PresetFilter

	// Preset default sorting
	PresetSort []*jsonapi.SortField

	// Preset default limit offset
	PresetPaginate *jsonapi.Pagination

	// RelationPrecheckPairs are the prechecks for the GetRelated and GetRelationship root
	RelationPrecheckPairs map[string]*RelationPresetRules

	Middlewares []MiddlewareFunc

	// GetModified defines if the result for Patch Should be returned.
	GetModifiedResult bool

	// CountList is a flag that defines if the List result should include objects count
	CountList bool

	// CustomHandlerFunc is a http.HandlerFunc defined for this endpoint
	CustomHandlerFunc http.HandlerFunc
}

func (e *Endpoint) HasPrechecks() bool {
	return len(e.PrecheckFilters) > 0 || len(e.PrecheckFilters) > 0
}

func (e *Endpoint) HasPresets() bool {
	return len(e.PresetPairs) > 0 || len(e.PresetFilters) > 0
}

type RelationPresetRules struct {
	// PrecheckPairs are the pairs of jsonapi.Scope and jsonapi.Filter
	// The scope deines the model from where the preset values should be taken
	// The second defines the filter field for the target model's scope that would be filled with
	// the values from the precheckpair scope
	PrecheckPairs []*jsonapi.PresetPair

	// PresetPairs are the paris of jsonapi.Scope and jsonapiFilter
	// The scope defines the model from where the preset values should be taken. It should not be
	// the same as target model.
	// The second parameter defines the target model's field and it subfield where the value
	// should be preset.
	PresetPairs []*jsonapi.PresetPair

	// PresetFilters are the filters for the target model's scope
	// They should be filled with values taken from context with key "jsonapi.PresetFilterValue"
	// When the values are taken the target model's scope would save the value for the relation
	// provided in the filterfield.
	PresetFilters []*jsonapi.PresetFilter

	// PrecheckFilters are the filters for the target model's scope
	// They should be filled with values taken from context with key "jsonapi.PrecheckPairFilterValue"
	// When the values are taken and saved into the precheck filter, the filter is added into the
	// target model's scope.
	PrecheckFilters []*jsonapi.PresetFilter

	// Preset default sorting
	PresetSort []*jsonapi.SortField

	// Preset default limit offset
	PresetPaginate *jsonapi.Pagination
}

func (e *Endpoint) String() string {
	return e.Type.String()
}
