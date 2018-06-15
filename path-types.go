package jsonapisdk

type EndpointType int

const (
	UnkownPath EndpointType = iota
	Create
	Get
	List
	Patch
	PatchAll
	Delete
	DeleteAll
)

func (e EndpointType) String() string {
	var op string
	switch e {
	case Create:
		op = "CREATE"
	case Get:
		op = "GET"
	case List:
		op = "LIST"
	case Patch:
		op = "PATCH"
	case Delete:
		op = "DELETE"

	default:
		op = "UNKNOWN"
	}
	return op
}

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
