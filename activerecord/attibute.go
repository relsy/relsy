package activerecord

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
)

const (
	Int    = "int"
	String = "string"
)

// primaryKey must implement attributes that are primary keys.
type primaryKey interface {
	PrimaryKey() bool
}

type Attribute interface {
	AttributeName() string
	CastType() string
	Validator
}

// PrimaryKey makes any specified attribute a primary key.
type PrimaryKey struct {
	Attribute
}

// PrimaryKey always returns true.
func (p PrimaryKey) PrimaryKey() bool {
	return true
}

type IntAttr struct {
	Name      string
	Validates IntValidators
}

func (a IntAttr) AttributeName() string            { return a.Name }
func (a IntAttr) CastType() string                 { return Int }
func (a IntAttr) Validate(value interface{}) error { return a.Validates.Validate(value) }

type StringAttr struct {
	Name      string
	Validates StringValidators
}

func (a StringAttr) AttributeName() string            { return a.Name }
func (a StringAttr) CastType() string                 { return String }
func (a StringAttr) Validate(value interface{}) error { return a.Validates.Validate(value) }

// ErrUnknownAttribute is returned on attempt to assign unknown attribute to the
// ActiveRecord.
type ErrUnknownAttribute struct {
	RecordName string
	Attr       string
}

// Error returns a string representation of the error.
func (e *ErrUnknownAttribute) Error() string {
	return fmt.Sprintf("unknown attribute %q for %s", e.Attr, e.RecordName)
}

const (
	// default name of the primary key.
	defaultPrimaryKeyName = "id"
)

type attributesMap map[string]Attribute

func (m attributesMap) Copy() attributesMap {
	mm := make(attributesMap, len(m))
	for name, attr := range m {
		mm[name] = attr
	}
	return mm
}

// attributes of the ActiveRecord.
type attributes struct {
	recordName string
	primaryKey Attribute
	keys       map[string]Attribute
	values     map[string]interface{}
}

// newAttributes creates a new collection of attributes for the specified record.
func newAttributes(
	recordName string, attrs map[string]Attribute, values map[string]interface{},
) (attributes, error) {

	recordAttrs := attributes{
		recordName: recordName,
		keys:       attrs,
		values:     values,
	}
	for _, attr := range recordAttrs.keys {
		// Save the primary key attribute as a standalone property for
		// easier access to it.
		if pk, ok := attr.(primaryKey); ok && pk.PrimaryKey() {
			if recordAttrs.primaryKey != nil {
				return attributes{}, errors.New("multiple primary keys are not supported")
			}
			recordAttrs.primaryKey = attr
		}
	}

	// When the primary key attribute was not specified directly, generate
	// a new "id" integer attribute, ensure that the attribute with the same
	// name is not presented in the schema definition.
	if _, dup := recordAttrs.keys[defaultPrimaryKeyName]; dup {
		err := errors.Errorf("%q is an attribute, but not a primary key", defaultPrimaryKeyName)
		return attributes{}, err
	}
	if recordAttrs.primaryKey == nil {
		pk := PrimaryKey{Attribute: IntAttr{Name: defaultPrimaryKeyName}}
		recordAttrs.primaryKey = pk
		recordAttrs.keys[defaultPrimaryKeyName] = pk
	}

	// Enforce values within a record, all of them must be
	// presented in the specified list of attributes.
	for attrName := range recordAttrs.values {
		if _, ok := recordAttrs.keys[attrName]; !ok {

			err := &ErrUnknownAttribute{RecordName: recordName, Attr: attrName}
			return attributes{}, err
		}
	}

	return recordAttrs, nil
}

func (a *attributes) forEach(fn func(name string, value interface{})) {
	for name, value := range a.values {
		fn(name, value)
	}
}

func (a *attributes) PrimaryKey() string {
	return a.primaryKey.AttributeName()
}

// ID returns the primary key column's value.
func (a *attributes) ID() interface{} {
	return a.values[a.primaryKey.AttributeName()]
}

// AttributeNames return an array of names for the attributes available on this object.
func (a *attributes) AttributeNames() []string {
	names := make([]string, 0, len(a.keys))
	for name := range a.keys {
		names = append(names, name)
	}
	sort.StringSlice(names).Sort()
	return names
}

// HasAttribute returns true if the given table attribute is in the attribute map,
// otherwise false.
func (a *attributes) HasAttribute(attrName string) bool {
	_, ok := a.keys[attrName]
	return ok
}

// AssignAttribute allows to set attribute by the name.
//
// Method return an error when value does not pass validation of the attribute.
func (a *attributes) AssignAttribute(attrName string, val interface{}) error {
	attr, ok := a.keys[attrName]
	if !ok {
		return &ErrUnknownAttribute{RecordName: a.recordName, Attr: attrName}
	}
	// Ensure that attribute passes validation.
	if err := attr.Validate(val); err != nil {
		return err
	}

	if a.values == nil {
		a.values = make(map[string]interface{})
	}
	a.values[attrName] = val
	return nil
}

// AccessAttribute returns the value of the attribute identified by attrName.
func (a *attributes) AccessAttribute(attrName string) (val interface{}) {
	if !a.HasAttribute(attrName) {
		return nil
	}
	return a.values[attrName]
}

// AttributePresent returns true if the specified attribute has been set by the user
// or by a database and is not nil, otherwise false.
func (a *attributes) AttributePresent(attrName string) bool {
	if _, ok := a.keys[attrName]; !ok {
		return false
	}
	return a.values[attrName] != nil
}
