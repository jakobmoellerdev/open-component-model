package cel

import "testing"

func TestTypeCheck(t *testing.T) {
	if err := OcmCelEnv(); err != nil {
		t.Fatalf("%v", err)
	}

}
