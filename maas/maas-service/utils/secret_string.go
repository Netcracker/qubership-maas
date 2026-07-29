package utils

import "fmt"

// SecretString use this type for sensitive string data such password etc.
// String will be obfuscated on print and marshalling attempts
type SecretString string

func (acc SecretString) Format(state fmt.State, _ rune) {
	if _, err := fmt.Fprint(state, "***"); err != nil {
		return
	}
}
func (acc SecretString) MarshalJSON() ([]byte, error) {
	return []byte("\"***\""), nil
}
