// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package bundlechanges

import (
	"fmt"

	"gopkg.in/juju/charm.v6-unstable"
)

// FromData generates and returns the list of changes required to deploy the
// given bundle data. The changes are sorted by requirements, so that they can
// be applied in order. The bundle data is assumed to be already verified.
func FromData(data *charm.BundleData) []Change {
	cs := &changeset{}
	addedServices := handleServices(cs.add, data.Services)
	addedMachines := handleMachines(cs.add, data.Machines)
	handleRelations(cs.add, data.Relations, addedServices)
	handleUnits(cs.add, data.Services, addedServices, addedMachines)
	return cs.sorted()
}

// Change holds a single change required to deploy a bundle.
type Change interface {
	// Id returns the unique identifier for this change.
	Id() string
	// Requires returns the ids of all the changes that must
	// be applied before this one.
	Requires() []string
	// Method returns the action to be performed to apply this change.
	Method() string
	// GUIArgs returns positional arguments to pass to the method, suitable for
	// being JSON-serialized and sent to the Juju GUI.
	GUIArgs() []interface{}
	// setId is used to set the identifier for the change.
	setId(string)
}

// changeInfo holds information on a change, suitable for embedding into a more
// specific change type.
type changeInfo struct {
	id       string
	requires []string
	method   string
}

// Id implements Change.Id.
func (ch *changeInfo) Id() string {
	return ch.id
}

// Requires implements Change.Requires.
func (ch *changeInfo) Requires() []string {
	// Avoid returning a nil interface because so that avoid returning a slice
	// that will serialize to JSON null.
	if ch.requires == nil {
		return []string{}
	}
	return ch.requires
}

// Method implements Change.Method.
func (ch *changeInfo) Method() string {
	return ch.method
}

// setId implements Change.setId.
func (ch *changeInfo) setId(id string) {
	ch.id = id
}

// newAddCharmChange creates a new change for adding a charm.
func newAddCharmChange(params AddCharmParams, requires ...string) *AddCharmChange {
	return &AddCharmChange{
		changeInfo: changeInfo{
			requires: requires,
			method:   "addCharm",
		},
		Params: params,
	}
}

// AddCharmChange holds a change for adding a charm to the environment.
type AddCharmChange struct {
	changeInfo
	// Params holds parameters for adding a charm.
	Params AddCharmParams
}

// GUIArgs implements Change.GUIArgs.
func (ch *AddCharmChange) GUIArgs() []interface{} {
	return []interface{}{ch.Params.Charm}
}

// AddCharmParams holds parameters for adding a charm to the environment.
type AddCharmParams struct {
	// Charm holds the URL of the charm to be added.
	Charm string
}

// newAddMachineChange creates a new change for adding a machine or container.
func newAddMachineChange(params AddMachineParams, requires ...string) *AddMachineChange {
	return &AddMachineChange{
		changeInfo: changeInfo{
			requires: requires,
			method:   "addMachines",
		},
		Params: params,
	}
}

// AddMachineChange holds a change for adding a machine or container.
type AddMachineChange struct {
	changeInfo
	// Params holds parameters for adding a machine.
	Params AddMachineParams
}

// GUIArgs implements Change.GUIArgs.
func (ch *AddMachineChange) GUIArgs() []interface{} {
	options := AddMachineOptions{
		Series:        ch.Params.Series,
		Constraints:   ch.Params.Constraints,
		ContainerType: ch.Params.ContainerType,
		ParentId:      ch.Params.ParentId,
	}
	return []interface{}{options}
}

// AddMachineOptions holds GUI options for adding a machine or container.
type AddMachineOptions struct {
	// Series holds the machine OS series.
	Series string `json:"series,omitempty"`
	// Constraints holds the machine constraints.
	Constraints string `json:"constraints,omitempty"`
	// ContainerType holds the machine container type (like "lxc" or "kvm").
	ContainerType string `json:"containerType,omitempty"`
	// ParentId holds the id of the parent machine.
	ParentId string `json:"parentId,omitempty"`
}

// AddMachineParams holds parameters for adding a machine or container.
type AddMachineParams struct {
	// Series holds the optional machine OS series.
	Series string
	// Constraints holds the optional machine constraints.
	Constraints string
	// ContainerType optionally holds the type of the container (for instance
	// ""lxc" or kvm"). It is not specified for top level machines.
	ContainerType string
	// ParentId optionally holds a placeholder pointing to another machine
	// change or to a unit change. This value is only specified in the case
	// this machine is a container, in which case also ContainerType is set.
	ParentId string
}

// newAddRelationChange creates a new change for adding a relation.
func newAddRelationChange(params AddRelationParams, requires ...string) *AddRelationChange {
	return &AddRelationChange{
		changeInfo: changeInfo{
			requires: requires,
			method:   "addRelation",
		},
		Params: params,
	}
}

// AddRelationChange holds a change for adding a relation between two services.
type AddRelationChange struct {
	changeInfo
	// Params holds parameters for adding a relation.
	Params AddRelationParams
}

// GUIArgs implements Change.GUIArgs.
func (ch *AddRelationChange) GUIArgs() []interface{} {
	return []interface{}{ch.Params.Endpoint1, ch.Params.Endpoint2}
}

// AddRelationParams holds parameters for adding a relation between two services.
type AddRelationParams struct {
	// Endpoint1 and Endpoint2 hold relation endpoints in the
	// "service:interface" form, where the service is always a placeholder
	// pointing to a service change, and the interface is optional. Examples
	// are "$deploy-42:web" or just "$deploy-42".
	Endpoint1 string
	Endpoint2 string
}

// newAddServiceChange creates a new change for adding a service.
func newAddServiceChange(params AddServiceParams, requires ...string) *AddServiceChange {
	return &AddServiceChange{
		changeInfo: changeInfo{
			requires: requires,
			method:   "deploy",
		},
		Params: params,
	}
}

// AddServiceChange holds a change for deploying a Juju service.
type AddServiceChange struct {
	changeInfo
	// Params holds parameters for adding a service.
	Params AddServiceParams
}

// GUIArgs implements Change.GUIArgs.
func (ch *AddServiceChange) GUIArgs() []interface{} {
	options := ch.Params.Options
	if options == nil {
		options = make(map[string]interface{}, 0)
	}
	storage := ch.Params.Storage
	if storage == nil {
		storage = make(map[string]string, 0)
	}
	return []interface{}{ch.Params.Charm, ch.Params.Service, options, ch.Params.Constraints, storage}
}

// AddServiceParams holds parameters for deploying a Juju service.
type AddServiceParams struct {
	// Charm holds the URL of the charm to be used to deploy this service.
	Charm string
	// Service holds the service name.
	Service string
	// Options holds service options.
	Options map[string]interface{}
	// Constraints holds the optional service constraints.
	Constraints string
	// Storage holds the optional storage constraints.
	Storage map[string]string
}

// newAddUnitChange creates a new change for adding a service unit.
func newAddUnitChange(params AddUnitParams, requires ...string) *AddUnitChange {
	return &AddUnitChange{
		changeInfo: changeInfo{
			requires: requires,
			method:   "addUnit",
		},
		Params: params,
	}
}

// AddUnitChange holds a change for adding a service unit.
type AddUnitChange struct {
	changeInfo
	// Params holds parameters for adding a unit.
	Params AddUnitParams
}

// GUIArgs implements Change.GUIArgs.
func (ch *AddUnitChange) GUIArgs() []interface{} {
	args := []interface{}{ch.Params.Service, nil}
	if ch.Params.To != "" {
		args[1] = ch.Params.To
	}
	return args
}

// AddUnitParams holds parameters for adding a service unit.
type AddUnitParams struct {
	// Service holds the service placeholder name for which a unit is added.
	Service string
	// To holds the optional location where to add the unit, as a placeholder
	// pointing to another unit change or to a machine change.
	To string
}

// newExposeChange creates a new change for exposing a service.
func newExposeChange(params ExposeParams, requires ...string) *ExposeChange {
	return &ExposeChange{
		changeInfo: changeInfo{
			requires: requires,
			method:   "expose",
		},
		Params: params,
	}
}

// ExposeChange holds a change for exposing a service.
type ExposeChange struct {
	changeInfo
	// Params holds parameters for exposing a service.
	Params ExposeParams
}

// GUIArgs implements Change.GUIArgs.
func (ch *ExposeChange) GUIArgs() []interface{} {
	return []interface{}{ch.Params.Service}
}

// ExposeParams holds parameters for exposing a service.
type ExposeParams struct {
	// Service holds the placeholder name of the service that must be exposed.
	Service string
}

// newSetAnnotationsChange creates a new change for setting annotations.
func newSetAnnotationsChange(params SetAnnotationsParams, requires ...string) *SetAnnotationsChange {
	return &SetAnnotationsChange{
		changeInfo: changeInfo{
			requires: requires,
			method:   "setAnnotations",
		},
		Params: params,
	}
}

// SetAnnotationsChange holds a change for setting service and machine
// annotations.
type SetAnnotationsChange struct {
	changeInfo
	// Params holds parameters for setting annotations.
	Params SetAnnotationsParams
}

// GUIArgs implements Change.GUIArgs.
func (ch *SetAnnotationsChange) GUIArgs() []interface{} {
	return []interface{}{ch.Params.Id, string(ch.Params.EntityType), ch.Params.Annotations}
}

// EntityType holds entity types ("service" or "machine").
type EntityType string

const (
	ServiceType EntityType = "service"
	MachineType EntityType = "machine"
)

// SetAnnotationsParams holds parameters for setting annotations.
type SetAnnotationsParams struct {
	// Id is the placeholder for the service or machine change corresponding to
	// the entity to be annotated.
	Id string
	// EntityType holds the type of the entity, "service" or "machine".
	EntityType EntityType
	// Annotations holds the annotations as key/value pairs.
	Annotations map[string]string
}

// changeset holds the list of changes returned by FromData.
type changeset struct {
	changes []Change
}

// add adds the given change to this change set.
func (cs *changeset) add(change Change) {
	change.setId(fmt.Sprintf("%s-%d", change.Method(), len(cs.changes)))
	cs.changes = append(cs.changes, change)
}

// sorted returns the changes sorted by requirements, required first.
func (cs *changeset) sorted() []Change {
	numChanges := len(cs.changes)
	records := make(map[string]bool, numChanges)
	sorted := make([]Change, 0, numChanges)
	changes := make([]Change, numChanges, numChanges*2)
	copy(changes, cs.changes)
mainloop:
	for len(changes) != 0 {
		// Note that all valid bundles have at least two changes
		// (add one charm and deploy one service).
		change := changes[0]
		changes = changes[1:]
		for _, r := range change.Requires() {
			if !records[r] {
				// This change requires a change which is not yet listed.
				// Push this change at the end of the list and retry later.
				changes = append(changes, change)
				continue mainloop
			}
		}
		records[change.Id()] = true
		sorted = append(sorted, change)
	}
	return sorted
}
