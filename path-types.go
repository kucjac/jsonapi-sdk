package jsonapisdk

type EndpointHandler int

const (
	UnkownPath EndpointHandler = iota
	Create
	Get
	GetRelated
	GetRelationship
	List
	Patch
	Delete
)

type Handlers []EndpointHandler

var (
	FullCRUD Handlers = []EndpointHandler{
		Create,
		Get,
		GetRelated,
		GetRelationship,
		List,
		Patch,
		Delete,
	}

	ReadOnly Handlers = []EndpointHandler{
		Get,
		GetRelated,
		GetRelationship,
		List,
	}

	CreateReadUpdate Handlers = []EndpointHandler{
		Create,
		Get,
		GetRelated,
		GetRelationship,
		List,
		Patch,
	}

	CreateRead Handlers = []EndpointHandler{
		Create,
		Get,
		GetRelated,
		GetRelationship,
		List,
	}
)
