package jsonapisdk

type EndpointType int

const (
	UnkownPath EndpointType = iota
	Create
	Get
	List
	Patch
	Delete
)

var (
	FullCRUD = []EndpointType{
		Create,
		Get,
		List,
		Patch,
		Delete,
	}

	ReadOnly = []EndpointType{
		Get,
		List,
	}

	CreateReadUpdate = []EndpointType{
		Create,
		Get,
		List,
		Patch,
	}

	CreateRead = []EndpointType{
		Create,
		Get,
		List,
	}
)
