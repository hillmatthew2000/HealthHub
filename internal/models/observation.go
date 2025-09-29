package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Observation represents a FHIR-inspired Observation resource (lab results)
type Observation struct {
	ID                string            `json:"id" gorm:"primaryKey"`
	Status            string            `json:"status" validate:"oneof=registered preliminary final amended corrected cancelled entered-in-error unknown"`
	Category          []Category        `json:"category" gorm:"serializer:json"`
	Code              CodeableConcept   `json:"code" gorm:"embedded"`
	Subject           Reference         `json:"subject" gorm:"embedded"`
	Encounter         *Reference        `json:"encounter,omitempty" gorm:"embedded;embeddedPrefix:encounter_"`
	EffectiveDateTime time.Time         `json:"effectiveDateTime"`
	Issued            *time.Time        `json:"issued,omitempty"`
	Performer         []Reference       `json:"performer,omitempty" gorm:"serializer:json"`
	ValueQuantity     *Quantity         `json:"valueQuantity,omitempty" gorm:"embedded;embeddedPrefix:value_quantity_"`
	ValueCodeable     *CodeableConcept  `json:"valueCodeableConcept,omitempty" gorm:"embedded;embeddedPrefix:value_codeable_"`
	ValueString       string            `json:"valueString,omitempty"`
	ValueBoolean      *bool             `json:"valueBoolean,omitempty"`
	ValueInteger      *int              `json:"valueInteger,omitempty"`
	ValueRange        *Range            `json:"valueRange,omitempty" gorm:"embedded;embeddedPrefix:value_range_"`
	ValueRatio        *Ratio            `json:"valueRatio,omitempty" gorm:"embedded;embeddedPrefix:value_ratio_"`
	ValueTime         *time.Time        `json:"valueTime,omitempty"`
	ValueDateTime     *time.Time        `json:"valueDateTime,omitempty"`
	ValuePeriod       *Period           `json:"valuePeriod,omitempty" gorm:"embedded;embeddedPrefix:value_period_"`
	DataAbsentReason  *CodeableConcept  `json:"dataAbsentReason,omitempty" gorm:"embedded;embeddedPrefix:absent_reason_"`
	Interpretation    []CodeableConcept `json:"interpretation,omitempty" gorm:"serializer:json"`
	Note              []Annotation      `json:"note,omitempty" gorm:"serializer:json"`
	BodySite          *CodeableConcept  `json:"bodySite,omitempty" gorm:"embedded;embeddedPrefix:body_site_"`
	Method            *CodeableConcept  `json:"method,omitempty" gorm:"embedded;embeddedPrefix:method_"`
	Specimen          *Reference        `json:"specimen,omitempty" gorm:"embedded;embeddedPrefix:specimen_"`
	Device            *Reference        `json:"device,omitempty" gorm:"embedded;embeddedPrefix:device_"`
	ReferenceRange    []ReferenceRange  `json:"referenceRange,omitempty" gorm:"serializer:json"`
	Component         []Component       `json:"component,omitempty" gorm:"serializer:json"`
	CreatedAt         time.Time         `json:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
	CreatedBy         string            `json:"createdBy"`
}

// Category represents an observation category
type Category struct {
	Coding []Coding `json:"coding" validate:"required,min=1"`
	Text   string   `json:"text,omitempty"`
}

// CodeableConcept represents a concept that may be coded
type CodeableConcept struct {
	Coding []Coding `json:"coding,omitempty" gorm:"serializer:json"`
	Text   string   `json:"text,omitempty"`
}

// Coding represents a code from a coding system
type Coding struct {
	System       string `json:"system,omitempty"`
	Version      string `json:"version,omitempty"`
	Code         string `json:"code,omitempty"`
	Display      string `json:"display,omitempty"`
	UserSelected *bool  `json:"userSelected,omitempty"`
}

// Reference represents a reference to another resource
type Reference struct {
	Reference  string      `json:"reference,omitempty"`
	Type       string      `json:"type,omitempty"`
	Identifier *Identifier `json:"identifier,omitempty" gorm:"embedded;embeddedPrefix:identifier_"`
	Display    string      `json:"display,omitempty"`
}

// Identifier represents an identifier for a resource
type Identifier struct {
	Use      string           `json:"use,omitempty" validate:"omitempty,oneof=usual official temp secondary old"`
	Type     *CodeableConcept `json:"type,omitempty" gorm:"embedded;embeddedPrefix:type_"`
	System   string           `json:"system,omitempty"`
	Value    string           `json:"value,omitempty"`
	Period   *Period          `json:"period,omitempty" gorm:"embedded;embeddedPrefix:period_"`
	Assigner *Reference       `json:"assigner,omitempty" gorm:"embedded;embeddedPrefix:assigner_"`
}

// Quantity represents a measured amount
type Quantity struct {
	Value      float64 `json:"value,omitempty"`
	Comparator string  `json:"comparator,omitempty" validate:"omitempty,oneof=< <= >= > ad"`
	Unit       string  `json:"unit,omitempty"`
	System     string  `json:"system,omitempty"`
	Code       string  `json:"code,omitempty"`
}

// Range represents a range of values
type Range struct {
	Low  *Quantity `json:"low,omitempty" gorm:"embedded;embeddedPrefix:low_"`
	High *Quantity `json:"high,omitempty" gorm:"embedded;embeddedPrefix:high_"`
}

// Ratio represents a ratio of two quantities
type Ratio struct {
	Numerator   *Quantity `json:"numerator,omitempty" gorm:"embedded;embeddedPrefix:numerator_"`
	Denominator *Quantity `json:"denominator,omitempty" gorm:"embedded;embeddedPrefix:denominator_"`
}

// Annotation represents a text note
type Annotation struct {
	AuthorReference *Reference `json:"authorReference,omitempty"`
	AuthorString    string     `json:"authorString,omitempty"`
	Time            *time.Time `json:"time,omitempty"`
	Text            string     `json:"text" validate:"required"`
}

// ReferenceRange represents the reference range for an observation
type ReferenceRange struct {
	Low       *Quantity         `json:"low,omitempty"`
	High      *Quantity         `json:"high,omitempty"`
	Type      *CodeableConcept  `json:"type,omitempty"`
	AppliesTo []CodeableConcept `json:"appliesTo,omitempty"`
	Age       *Range            `json:"age,omitempty"`
	Text      string            `json:"text,omitempty"`
}

// Component represents a component observation
type Component struct {
	Code             CodeableConcept   `json:"code"`
	ValueQuantity    *Quantity         `json:"valueQuantity,omitempty"`
	ValueCodeable    *CodeableConcept  `json:"valueCodeableConcept,omitempty"`
	ValueString      string            `json:"valueString,omitempty"`
	ValueBoolean     *bool             `json:"valueBoolean,omitempty"`
	ValueInteger     *int              `json:"valueInteger,omitempty"`
	ValueRange       *Range            `json:"valueRange,omitempty"`
	ValueRatio       *Ratio            `json:"valueRatio,omitempty"`
	ValueTime        *time.Time        `json:"valueTime,omitempty"`
	ValueDateTime    *time.Time        `json:"valueDateTime,omitempty"`
	ValuePeriod      *Period           `json:"valuePeriod,omitempty"`
	DataAbsentReason *CodeableConcept  `json:"dataAbsentReason,omitempty"`
	Interpretation   []CodeableConcept `json:"interpretation,omitempty"`
	ReferenceRange   []ReferenceRange  `json:"referenceRange,omitempty"`
}

// BeforeCreate is a GORM hook that runs before creating an observation
func (o *Observation) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return nil
}

// TableName returns the table name for the Observation model
func (Observation) TableName() string {
	return "observations"
}

// IsAbnormal checks if the observation result is abnormal
func (o *Observation) IsAbnormal() bool {
	for _, interp := range o.Interpretation {
		for _, coding := range interp.Coding {
			switch coding.Code {
			case "A", "AA", "HH", "LL", "H", "L":
				return true
			}
		}
	}
	return false
}

// GetDisplayValue returns a human-readable display of the observation value
func (o *Observation) GetDisplayValue() string {
	if o.ValueQuantity != nil {
		unit := o.ValueQuantity.Unit
		if unit == "" {
			unit = o.ValueQuantity.Code
		}
		if unit != "" {
			return fmt.Sprintf("%.2f %s", o.ValueQuantity.Value, unit)
		}
		return fmt.Sprintf("%.2f", o.ValueQuantity.Value)
	}

	if o.ValueString != "" {
		return o.ValueString
	}

	if o.ValueBoolean != nil {
		if *o.ValueBoolean {
			return "True"
		}
		return "False"
	}

	if o.ValueInteger != nil {
		return fmt.Sprintf("%d", *o.ValueInteger)
	}

	if o.ValueCodeable != nil && o.ValueCodeable.Text != "" {
		return o.ValueCodeable.Text
	}

	return ""
}

// GetCodeDisplay returns a human-readable display of the observation code
func (o *Observation) GetCodeDisplay() string {
	if len(o.Code.Coding) > 0 {
		coding := o.Code.Coding[0]
		if coding.Display != "" {
			return coding.Display
		}
		if coding.Code != "" {
			return coding.Code
		}
	}

	if o.Code.Text != "" {
		return o.Code.Text
	}

	return "Unknown"
}
