package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DbURL           string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

func check(e error) {
	if e != nil {
		fmt.Println("Error:", e)
		os.Exit(1)
	}
}

func getFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	fmt.Println(homeDir)
	path := filepath.Join(homeDir, ".gatorconfig.json")
	clean := filepath.Clean(path)
	return filepath.FromSlash(clean), nil
}

// Reads file at userHomeDir/.gatorconfig.json and outputs Config struct
func Read() Config {
	path, err := getFilePath()
	check(err)

	fmt.Println(path)

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.MkdirAll(dir, 0755) // Create directory if missing
			check(err)
		} else {
			check(err)
		}
	}

	// Ensure the file exists, create it if missing
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			defaultConfig := `{"db_url":"postgres://postgres:postgres@localhost:5432/gator?sslmode=disable","current_user_name":""}`
			err = os.WriteFile(path, []byte(defaultConfig), 0644)
			check(err)
			fmt.Println("Config file created:", path)
		} else {
			check(err)
		}
	}

	// Double-check if the file now exists
	if _, err := os.Stat(path); err != nil {
		fmt.Println("Critical: Config file still does not exist after creation attempt!")
		check(err)
	}

	// Read the config file
	data, err := os.ReadFile(path)
	check(err)

	// Initialize Config struct
	var config Config

	// Unmarshal the JSON into the config struct
	err = json.Unmarshal(data, &config)
	check(err)

	return config
}

func (c Config) SetUser(name string) {
	//set field to name
	c.CurrentUserName = name
	//set path to config json
	path, err := getFilePath()
	check(err)
	// remarshal the json
	data, err := json.Marshal(c)
	check(err)
	//write the file
	err = os.WriteFile(path, data, 0644)
	check(err)
}
