package types

import "fmt"

type Gender int8

const (
	GenderMale   Gender = 1
	GenderFemale Gender = 2
)

func (g Gender) Validate() error {
	switch g {
	case GenderMale, GenderFemale:
		return nil
	default:
		return fmt.Errorf("invalid gender: %d", g)
	}
}
