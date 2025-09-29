package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Patient represents a FHIR-inspired Patient resource
type Patient struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	Active    bool      `json:"active" gorm:"default:true"`
	Name      []Name    `json:"name" gorm:"serializer:json"`
	Gender    string    `json:"gender" validate:"oneof=male female other unknown"`
	BirthDate time.Time `json:"birthDate"`
	Telecom   []Contact `json:"telecom" gorm:"serializer:json"`
	Address   []Address `json:"address" gorm:"serializer:json"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedBy string    `json:"createdBy"`
}

// Name represents a person's name following FHIR structure
type Name struct {
	Use    string   `json:"use" validate:"oneof=usual official temp nickname anonymous old"`
	Family string   `json:"family" validate:"required"`
	Given  []string `json:"given" validate:"required,min=1"`
	Prefix []string `json:"prefix,omitempty"`
	Suffix []string `json:"suffix,omitempty"`
}

// Contact represents contact information (phone, email, etc.)
type Contact struct {
	System string `json:"system" validate:"oneof=phone fax email pager url sms other"`
	Value  string `json:"value" validate:"required"`
	Use    string `json:"use" validate:"oneof=home work temp old mobile"`
	Rank   int    `json:"rank,omitempty"`
}

// Address represents a physical address
type Address struct {
	Use        string   `json:"use" validate:"oneof=home work temp old billing"`
	Type       string   `json:"type,omitempty" validate:"omitempty,oneof=postal physical both"`
	Text       string   `json:"text,omitempty"`
	Line       []string `json:"line,omitempty"`
	City       string   `json:"city,omitempty"`
	District   string   `json:"district,omitempty"`
	State      string   `json:"state,omitempty"`
	PostalCode string   `json:"postalCode,omitempty"`
	Country    string   `json:"country,omitempty"`
	Period     *Period  `json:"period,omitempty"`
}

// Period represents a time period with start and end
type Period struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

// BeforeCreate is a GORM hook that runs before creating a patient
func (p *Patient) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// TableName returns the table name for the Patient model
func (Patient) TableName() string {
	return "patients"
}

// GetFullName returns the patient's full name
func (p *Patient) GetFullName() string {
	if len(p.Name) == 0 {
		return ""
	}

	name := p.Name[0] // Use the first name entry
	fullName := ""

	// Add prefixes
	for _, prefix := range name.Prefix {
		if fullName != "" {
			fullName += " "
		}
		fullName += prefix
	}

	// Add given names
	for _, given := range name.Given {
		if fullName != "" {
			fullName += " "
		}
		fullName += given
	}

	// Add family name
	if name.Family != "" {
		if fullName != "" {
			fullName += " "
		}
		fullName += name.Family
	}

	// Add suffixes
	for _, suffix := range name.Suffix {
		if fullName != "" {
			fullName += " "
		}
		fullName += suffix
	}

	return fullName
}

// GetPrimaryEmail returns the patient's primary email address
func (p *Patient) GetPrimaryEmail() string {
	for _, contact := range p.Telecom {
		if contact.System == "email" && (contact.Use == "home" || contact.Use == "work") {
			return contact.Value
		}
	}
	return ""
}

// GetPrimaryPhone returns the patient's primary phone number
func (p *Patient) GetPrimaryPhone() string {
	for _, contact := range p.Telecom {
		if contact.System == "phone" && (contact.Use == "home" || contact.Use == "mobile") {
			return contact.Value
		}
	}
	return ""
}
