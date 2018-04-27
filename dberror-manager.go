package jsonapisdk

import (
	"errors"
	"github.com/kucjac/jsonapi"
	"github.com/kucjac/uni-db"
	"sync"
)

// DefaultErrorMap contain default mapping of unidb.Error prototype into
// jsonapi.Error. It is used by default by 'ErrorManager' if created using New() function.
var DefaultErrorMap map[unidb.Error]jsonapi.ErrorObject = map[unidb.Error]jsonapi.ErrorObject{
	unidb.ErrNoResult:              jsonapi.ErrResourceNotFound,
	unidb.ErrConnExc:               jsonapi.ErrInternalError,
	unidb.ErrCardinalityViolation:  jsonapi.ErrInternalError,
	unidb.ErrDataException:         jsonapi.ErrInvalidInput,
	unidb.ErrIntegrConstViolation:  jsonapi.ErrInvalidInput,
	unidb.ErrRestrictViolation:     jsonapi.ErrInvalidInput,
	unidb.ErrNotNullViolation:      jsonapi.ErrInvalidInput,
	unidb.ErrForeignKeyViolation:   jsonapi.ErrInvalidInput,
	unidb.ErrUniqueViolation:       jsonapi.ErrResourceAlreadyExists,
	unidb.ErrCheckViolation:        jsonapi.ErrInvalidInput,
	unidb.ErrInvalidTransState:     jsonapi.ErrInternalError,
	unidb.ErrInvalidTransTerm:      jsonapi.ErrInternalError,
	unidb.ErrTransRollback:         jsonapi.ErrInternalError,
	unidb.ErrTxDone:                jsonapi.ErrInternalError,
	unidb.ErrInvalidAuthorization:  jsonapi.ErrInsufficientAccPerm,
	unidb.ErrInvalidPassword:       jsonapi.ErrInternalError,
	unidb.ErrInvalidSchemaName:     jsonapi.ErrInternalError,
	unidb.ErrInvalidSyntax:         jsonapi.ErrInternalError,
	unidb.ErrInsufficientPrivilege: jsonapi.ErrInsufficientAccPerm,
	unidb.ErrInsufficientResources: jsonapi.ErrInternalError,
	unidb.ErrProgramLimitExceeded:  jsonapi.ErrInternalError,
	unidb.ErrSystemError:           jsonapi.ErrInternalError,
	unidb.ErrInternalError:         jsonapi.ErrInternalError,
	unidb.ErrUnspecifiedError:      jsonapi.ErrInternalError,
}

// ErrorManager defines the database unidb.Error one-to-one mapping
// into jsonapi.Error. The default error mapping is defined
// in package variable 'DefaultErrorMap'.
//
type ErrorManager struct {
	dbToRest map[unidb.Error]jsonapi.ErrorObject
	sync.RWMutex
}

// NewErrorMapper creates new error handler with already inited ErrorMap
func NewDBErrorMgr() *ErrorManager {
	return &ErrorManager{dbToRest: DefaultErrorMap}
}

// Handle enables unidb.Error handling so that proper jsonapi.ErrorObject is returned.
// It returns jsonapi.ErrorObject if given database error exists in the private error mapping.
// If provided dberror doesn't have prototype or no mapping exists for given unidb.Error an
// application 'error' would be returned.
// Thread safety by using RWMutex.RLock
func (r *ErrorManager) Handle(dberr *unidb.Error) (*jsonapi.ErrorObject, error) {
	// Get the prototype for given dberr
	dbProto, err := dberr.GetPrototype()
	if err != nil {
		return nil, err
	}

	// Get Rest
	r.RLock()
	apierr, ok := r.dbToRest[dbProto]
	r.RUnlock()
	if !ok {
		err = errors.New("Given database error is unrecognised by the handler")
		return nil, err
	}

	// // Create new entity
	return &apierr, nil
}

// LoadCustomErrorMap enables replacement of the ErrorManager default error map.
// This operation is thread safe - with RWMutex.Lock
func (r *ErrorManager) LoadCustomErrorMap(errorMap map[unidb.Error]jsonapi.ErrorObject) {
	r.Lock()
	r.dbToRest = errorMap
	r.Unlock()
}

// UpdateErrorMapEntry changes single entry in the Error Handler error map.
// This operation is thread safe - with RWMutex.Lock
func (r *ErrorManager) UpdateErrorEntry(
	dberr unidb.Error,
	apierr jsonapi.ErrorObject,
) {
	r.Lock()
	r.dbToRest[dberr] = apierr
	r.Unlock()
}
