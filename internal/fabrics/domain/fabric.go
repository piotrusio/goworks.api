package domain

import (
	"errors"
	"regexp"
)

var (
	ErrInvalidFabricCodeLength  = errors.New("the fabric code length must be 2-30")
	ErrInvalidFabricCodePattern = errors.New("the fabric code can contain A-Z and 0-9 characters")
	ErrInvalidFabricNameLength  = errors.New("the fabric name length must be 1-250")
	ErrRecordNotFound           = errors.New("record not found")
	ErrDuplicateFabricCode      = errors.New("a fabric with this code already exsists")
	ErrConcurrencyConflict      = errors.New("a concurrency conflict occurred")
	ErrFabricDeleted            = errors.New("cannot perform on a deleted fabric")
)

const (
	StatusActive  = "ACTIVE"
	StatusDeleted = "DELETED"
)

type Event any

type Fabric struct {
	Code        string
	Name        string
	MeasureUnit string
	OfferStatus string
	Status      string
	Version     int
	events      []Event
}

type FabricCreated struct {
	Code        string
	Name        string
	MeasureUnit string
	OfferStatus string
	Version     int
}

type FabricUpdated struct {
	Code        string
	Name        string
	MeasureUnit string
	OfferStatus string
	Version     int
}

type FabricDeleted struct {
	Code    string
	Version int
}

type FabricReactivated struct {
	Code        string
	Name        string
	MeasureUnit string
	OfferStatus string
	Version     int
}

func NewFabric(code, name, measureUnit, offerStatus string) (*Fabric, error) {
	if err := validateCode(code); err != nil {
		return nil, err
	}
	if err := validateName(name); err != nil {
		return nil, err
	}

	fabric := &Fabric{
		Code:        code,
		Name:        name,
		MeasureUnit: measureUnit,
		OfferStatus: offerStatus,
		Status:      StatusActive,
		Version:     1,
	}

	event := FabricCreated{
		Code:        fabric.Code,
		Name:        fabric.Name,
		MeasureUnit: fabric.MeasureUnit,
		OfferStatus: fabric.OfferStatus,
		Version:     fabric.Version,
	}

	fabric.events = append(fabric.events, event)
	return fabric, nil
}

func (f *Fabric) UpdateFabric(name, measureUnit, offerStatus string, version int) error {
	// Soft delete check
	if f.Status == StatusDeleted {
		return ErrFabricDeleted
	}
	// Optimistic concurrency check
	if f.Version != version {
		return ErrConcurrencyConflict
	}
	if err := validateName(name); err != nil {
		return err
	}

	f.Name = name
	f.MeasureUnit = measureUnit
	f.OfferStatus = offerStatus
	f.Version++ // Increment version on successful update

	event := FabricUpdated{
		Code:        f.Code,
		Name:        f.Name,
		MeasureUnit: f.MeasureUnit,
		OfferStatus: f.OfferStatus,
		Version:     f.Version,
	}

	f.events = append(f.events, event)
	return nil
}

func (f *Fabric) Delete(version int) error {
	if f.Status == StatusDeleted {
		return ErrFabricDeleted
	}
	if f.Version != version {
		return ErrConcurrencyConflict
	}

	f.Status = StatusDeleted
	f.Version++

	event := FabricDeleted{
		Code:    f.Code,
		Version: f.Version,
	}
	f.events = append(f.events, event)

	return nil
}

func (f *Fabric) Reactivate(name, measureUnit, offerStatus string, version int) error {
	if f.Status == StatusActive {
		// if it's already active, this shold be treated as a regular update
		return f.UpdateFabric(name, measureUnit, offerStatus, version)
	}
	if f.Version != version {
		return ErrConcurrencyConflict
	}
	if err := validateName(name); err != nil {
		return err
	}

	f.Status = StatusActive
	f.Name = name
	f.MeasureUnit = measureUnit
	f.OfferStatus = offerStatus
	f.Version++

	event := FabricReactivated{
		Code:        f.Code,
		Name:        f.Name,
		MeasureUnit: f.MeasureUnit,
		OfferStatus: f.OfferStatus,
		Version:     f.Version,
	}
	f.events = append(f.events, event)

	return nil
}

func (f *Fabric) Events() []Event {
	return f.events
}

func validateCode(code string) error {
	if len(code) < 2 || len(code) > 30 {
		return ErrInvalidFabricCodeLength
	}
	pattern := regexp.MustCompile(`^[A-Z0-9]+$`)
	if !pattern.MatchString(code) {
		return ErrInvalidFabricCodePattern
	}
	return nil
}

func validateName(name string) error {
	if len(name) < 1 || len(name) > 250 {
		return ErrInvalidFabricNameLength
	}
	return nil
}
