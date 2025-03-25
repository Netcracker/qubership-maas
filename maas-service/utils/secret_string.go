package utils

import "fmt"

// SecretString use this type for sensitive string data such password etc.
// String will be obfuscated on print and marshalling attempts
type SecretString string

func (acc SecretString) Format(state fmt.State, _ int32) {
	fmt.Fprintf(state, "***")
}
func (acc SecretString) MarshalJSON() ([]byte, error) {
	return []byte("\"***\""), nil
}
