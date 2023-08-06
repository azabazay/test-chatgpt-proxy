package main

import (
	"fmt"
	"os"
)

func GetEnv(key string) (string, error) {
	envVar := os.Getenv(key)

	if envVar == "" {
		return "", fmt.Errorf("Environment variable is not set for key: %s", key)
	}

	return envVar, nil
}
