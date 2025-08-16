package main

import (
	"os"
	"testing"
)

func TestEnvExampleFileExists(t *testing.T) {
	if _, err := os.Stat(".env.example"); os.IsNotExist(err) {
		t.Errorf(".env.example file not found. This file is required for configuration.")
	}
}
