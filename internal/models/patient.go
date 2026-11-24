package models

import (
	"strings"
	"time"
)

// Patient represents a FHIR-inspired Patient resource.
type Patient struct {
	ID        string    `json:"id"`
	Active    bool      `json:"active"`
	Name      []Name    `json:"name" validate:"required,min=1,max=10,dive"`
	Gender    string    `json:"gender" validate:"required,oneof=male female other unknown"`
	BirthDate *Date     `json:"birthDate,omitempty"`
	Telecom   []Contact `json:"telecom,omitempty" validate:"max=20,dive"`
	Address   []Address `json:"address,omitempty" validate:"max=10,dive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedBy string    `json:"createdBy,omitempty"`
}

// Name represents a person's name following the FHIR HumanName structure.
type Name struct {
	Use    string   `json:"use,omitempty" validate:"omitempty,oneof=usual official temp nickname anonymous old maiden"`
	Family string   `json:"family" validate:"required,max=200"`
	Given  []string `json:"given" validate:"required,min=1,max=10,dive,required,max=200"`
	Prefix []string `json:"prefix,omitempty" validate:"max=5,dive,max=50"`
	Suffix []string `json:"suffix,omitempty" validate:"max=5,dive,max=50"`
}

// Contact represents contact information (phone, email, etc.).
type Contact struct {
	System string `json:"system" validate:"required,oneof=phone fax email pager url sms other"`
	Value  string `json:"value" validate:"required,max=254"`
	Use    string `json:"use,omitempty" validate:"omitempty,oneof=home work temp old mobile"`
	Rank   int    `json:"rank,omitempty" validate:"min=0"`
}

// Address represents a physical address.
type Address struct {
	Use        string   `json:"use,omitempty" validate:"omitempty,oneof=home work temp old billing"`
	Type       string   `json:"type,omitempty" validate:"omitempty,oneof=postal physical both"`
	Text       string   `json:"text,omitempty" validate:"max=500"`
	Line       []string `json:"line,omitempty" validate:"max=5,dive,max=200"`
	City       string   `json:"city,omitempty" validate:"max=100"`
	District   string   `json:"district,omitempty" validate:"max=100"`
	State      string   `json:"state,omitempty" validate:"max=100"`
	PostalCode string   `json:"postalCode,omitempty" validate:"max=20"`
	Country    string   `json:"country,omitempty" validate:"max=100"`
	Period     *Period  `json:"period,omitempty"`
}

// Period represents a time period with start and end.
type Period struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

// FullName returns the patient's first recorded name as display text.
func (p *Patient) FullName() string {
	if len(p.Name) == 0 {
		return ""
	}
	n := p.Name[0]
	parts := make([]string, 0, len(n.Prefix)+len(n.Given)+1+len(n.Suffix))
	parts = append(parts, n.Prefix...)
	parts = append(parts, n.Given...)
	if n.Family != "" {
		parts = append(parts, n.Family)
	}
	parts = append(parts, n.Suffix...)
	return strings.Join(parts, " ")
}
