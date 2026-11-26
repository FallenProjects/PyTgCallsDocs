package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	APIID   int32
	APIHash string
	Token   string
}

func required(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func Load() (Config, error) {
	value, err := required("API_ID")
	if err != nil {
		return Config{}, err
	}
	id, err := strconv.ParseInt(value, 10, 32)
	if err != nil || id <= 0 {
		return Config{}, fmt.Errorf("API_ID must be a positive 32-bit integer")
	}
	hash, err := required("API_HASH")
	if err != nil {
		return Config{}, err
	}
	token, err := required("TOKEN")
	if err != nil {
		return Config{}, err
	}
	return Config{APIID: int32(id), APIHash: hash, Token: token}, nil
}
