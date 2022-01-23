package activerecord

import (
	"fmt"
	"sort"
	"strings"

	. "github.com/activegraph/activegraph/activesupport"
)

type ErrAssociation struct {
	Message string
}

func (e ErrAssociation) Error() string {
	return e.Message
}

type ErrUnknownAssociation struct {
	RecordName string
	Assoc      string
}

func (e ErrUnknownAssociation) Error() string {
	return fmt.Sprintf("unknown association %q for %s", e.Assoc, e.RecordName)
}

type Association interface {
	// AssociationOwner() *Relation
	AssociationName() string
	AssociationForeignKey() string
}

type SingularAssociation interface {
	Association
	AccessAssociation(owner *ActiveRecord) RecordResult
}

type CollectionAssociation interface {
	Association
	AccessCollection(owner *ActiveRecord) CollectionResult
}

type AssociationMethods interface {
	AssociationNames() []string
	HasAssociation(assocName string) bool
	HasAssociations(assocNames ...string) bool
	ReflectOnAssociation(assocName string) *AssociationReflection
	ReflectOnAllAssociations() []*AssociationReflection
	// AssociationForInspect(assocName string) Association
	// AssociationsForInspect(assocNames ...string) []Association
}

type AssociationAccessors interface {
	// AssignAssociation(assocName string, assoc *ActiveRecord) error
	Association(assocName string) RecordResult
	AccessAssociation(assocName string) (*ActiveRecord, error)
}

type CollectionAccessors interface {
	// AssignCollection(collName string, coll []*ActiveRecord) error
	Collection(collName string) CollectionResult
	AccessCollection(collName string) (*Relation, error)
}

type AssociationReflection struct {
	*Relation
	Association
}

type BelongsTo struct {
	owner      *Relation
	reflection *Reflection
	targetName string
	foreignKey string
}

func (a *BelongsTo) AssociationOwner() *Relation {
	return a.owner
}

func (a *BelongsTo) AssociationName() string {
	return a.targetName
}

// ForeignKey sets the foreign key used for the association. By default this is
// guessed to be the name of this relation in lower-case and "_id" suffixed.
//
// So a relation that defines a BelongsTo("person") association will use "person_id"
// as a default foreign key.
func (a *BelongsTo) ForeignKey(fk string) {
	a.foreignKey = fk
}

func (a *BelongsTo) AssociationForeignKey() string {
	if a.foreignKey != "" {
		return a.foreignKey
	}
	// target_id
	return a.targetName + "_" + defaultPrimaryKeyName
}

// AccessAssociation returns a record of the target.
//
//	activerecord.New("owner", func(r *activerecord.R) {
//		r.BelongsTo("target")
//	})
//
// This association considers the following tables relation:
//
//	+------------------------+        +----------------+
//	|          owners        |        |     targets    |
//	+------------+-----------+        +------+---------+
//	| id         | integer   |    +-->| id   | integer | pk
//	| target_id  | string    |*---+   | name | string  |
// 	| updated_at | timestamp |        +------+---------+
//	+------------+-----------+
//
func (a *BelongsTo) AccessAssociation(owner *ActiveRecord) RecordResult {
	// Find target association relation given it's name.
	targets, err := a.reflection.Reflection(a.targetName)
	if err != nil {
		return ErrRecord(err)
	}

	targetId := owner.Attribute(a.AssociationForeignKey())
	return targets.WithContext(owner.Context()).Find(targetId)
}

func (a *BelongsTo) String() string {
	return fmt.Sprintf("#<Association type: 'belongs_to', name: '%s'>", a.targetName)
}

type HasMany struct {
	owner      *Relation
	reflection *Reflection
	targetName string
	foreignKey string
}

func (a *HasMany) AssociationName() string {
	return a.targetName
}

func (a *HasMany) AssociationForeignKey() string {
	// TODO: this is completely wrong.
	if a.foreignKey != "" {
		return a.foreignKey
	}
	return strings.ToLower(a.owner.Name()) + "_" + defaultPrimaryKeyName
}

// AccessCollection returns a collection of the target records.
//
// HasMany association indicates a one-to-many association with another model. The
// association indicates that each instance of the model has zero or more instances
// of target model.
//
//	activerecord.New("owner", func(r *activerecord.R) {
//		r.HasMany("targets")
//	})
//
// This association considers the following tables relation:
//
//	+----------------+         +--------------------+
//	|     owners     |         |       targets      |
//	+------+---------+         +----------+---------+
//	| id   | integer |<---+    | id       | integer |
//	| name | string  |    +---*| owner_id | integer |
//	+------+---------+         | name     | string  |
//	                           +----------+---------+
//
func (a *HasMany) AccessCollection(owner *ActiveRecord) CollectionResult {
	targets, err := a.reflection.Reflection(a.targetName)
	if err != nil {
		return CollectionResult{Err[*Relation](err)}
	}

	targets = targets.WithContext(owner.Context())

	// TODO: Make "scope" accessable and understandable.
	targets = targets.Where(a.AssociationForeignKey(), owner.ID())
	return CollectionResult{Ok(targets)}
}

func (a *HasMany) String() string {
	return fmt.Sprintf("#<Association type: 'has_many', name: '%s'>", a.targetName)
}

type HasOne struct {
	owner      *Relation
	reflection *Reflection
	targetName string
	foreignKey string
}

func (a *HasOne) AssociationOwner() *Relation {
	return a.owner
}

func (a *HasOne) AssociationName() string {
	if a.foreignKey != "" {
		return a.foreignKey
	}
	return a.targetName + "_" + defaultPrimaryKeyName
}

func (a *HasOne) AssociationForeignKey() string {
	// TODO: return actual table's primary key.
	return defaultPrimaryKeyName
}

// The association indicates that one model has a reference to this model.
// That "target" model can be fetched through this association.
//
//	activerecord.New("owner", func(r *activerecord.R) {
//		r.HasOne("target")
//	})
//
// This association considers the following tables relation:
//
//	+----------------+         +--------------------+
//	|     owners     |         |       targets      |
//	+------+---------+         +----------+---------+
//	| id   | integer |<---+    | id       | integer |
//	| name | string  |    +---*| owner_id | integer |
//	+------+---------+         | name     | string  |
//	                           +----------+---------+
//
func (a *HasOne) AccessAssociation(owner *ActiveRecord) RecordResult {
	// Find target association relation given it's name.
	targets, err := a.reflection.Reflection(a.targetName)
	if err != nil {
		return ErrRecord(err)
	}

	targets = targets.WithContext(owner.Context())
	targets = targets.Where(a.AssociationForeignKey(), owner.ID())

	records, err := targets.Limit(2).ToA()
	if err != nil {
		return ErrRecord(err)
	}
	switch len(records) {
	case 0:
		return OkRecord(nil)
	case 1:
		return OkRecord(records[0])
	default:
		return ErrRecord(ErrAssociation{
			fmt.Sprintf("declared 'has_one' association, but has many: %s", records),
		})
	}
}

func (a *HasOne) String() string {
	return fmt.Sprintf("#<Assocation type: 'has_one', name: '%s'>", a.targetName)
}

type associationsMap map[string]Association

func (m associationsMap) copy() associationsMap {
	mm := make(associationsMap, len(m))
	for name, assoc := range m {
		mm[name] = assoc
	}
	return mm
}

type associations struct {
	recordName string
	rec        *ActiveRecord
	reflection *Reflection
	keys       associationsMap
	values     map[string]*ActiveRecord
}

func newAssociations(
	recordName string, assocs associationsMap, reflection *Reflection,
) *associations {
	return &associations{
		recordName: recordName,
		reflection: reflection,
		keys:       assocs,
		values:     make(map[string]*ActiveRecord),
	}
}

func (a *associations) delegateAccessors(rec *ActiveRecord) *associations {
	a.rec = rec
	return a
}

func (a *associations) copy() *associations {
	values := make(map[string]*ActiveRecord, len(a.values))
	for k, v := range a.values {
		values[k] = v
	}
	return &associations{
		recordName: a.recordName,
		reflection: a.reflection,
		keys:       a.keys.copy(),
		values:     values,
	}
}

func (a *associations) HasAssociation(assocName string) bool {
	_, ok := a.keys[assocName]
	return ok
}

func (a *associations) HasAssociations(assocNames ...string) bool {
	for _, assocName := range assocNames {
		if !a.HasAssociation(assocName) {
			return false
		}
	}
	return true
}

func (a *associations) get(assocName string) Association {
	if !a.HasAssociation(assocName) {
		return nil
	}
	return a.keys[assocName]
}

// ReflectOnAssociation returns AssociationReflection for the specified association.
func (a *associations) ReflectOnAssociation(assocName string) *AssociationReflection {
	if !a.HasAssociation(assocName) {
		return nil
	}
	rel, err := a.reflection.Reflection(a.keys[assocName].AssociationName())
	if err != nil {
		return nil
	}
	return &AssociationReflection{Relation: rel, Association: a.keys[assocName]}
}

// ReflectOnAllAssociations returns an array of AssociationReflection types for all
// associations in the Relation.
func (a *associations) ReflectOnAllAssociations() []*AssociationReflection {
	arefs := make([]*AssociationReflection, 0, len(a.keys))
	for _, assoc := range a.keys {
		rel, _ := a.reflection.Reflection(assoc.AssociationName())
		if rel == nil {
			continue
		}
		arefs = append(arefs, &AssociationReflection{Relation: rel, Association: assoc})
	}
	return arefs
}

func (a *associations) AssociationNames() []string {
	names := make([]string, 0, len(a.keys))
	for name := range a.keys {
		names = append(names, name)
	}
	sort.StringSlice(names).Sort()
	return names
}

func (a *associations) Association(assocName string) RecordResult {
	assoc := a.get(assocName)
	if assoc == nil {
		return ErrRecord(ErrUnknownAssociation{RecordName: a.rec.Name(), Assoc: assocName})
	}

	if rec, ok := a.values[assocName]; ok {
		return OkRecord(rec)
	}

	sa, ok := assoc.(SingularAssociation)
	if !ok {
		message := fmt.Sprintf("'%s' is not a singular association", assocName)
		return ErrRecord(ErrAssociation{Message: message})
	}
	return sa.AccessAssociation(a.rec)
}

func (a *associations) AccessAssociation(assocName string) (*ActiveRecord, error) {
	assoc := a.Association(assocName)
	return assoc.Ok().UnwrapOr(nil), assoc.Err()
}

func (a *associations) Collection(collName string) CollectionResult {
	assoc := a.get(collName)
	if assoc == nil {
		return ErrCollection(ErrUnknownAssociation{RecordName: a.rec.Name(), Assoc: collName})
	}

	ca, ok := assoc.(CollectionAssociation)
	if !ok {
		message := fmt.Sprintf("'%s' is not a collection association", collName)
		return ErrCollection(ErrAssociation{Message: message})
	}
	return CollectionResult{ca.AccessCollection(a.rec)}
}

func (a *associations) AccessCollection(collName string) (*Relation, error) {
	collection := a.Collection(collName)
	return collection.Ok().UnwrapOr(nil), collection.Err()
}
