package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	DbURL           string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func getFilePath() (string, error) {
	//find home dir
	homeDir, err := os.UserHomeDir()
	check(err)
	//set path to config json
	path := homeDir + "/.gatorconfig.json"

	return path, nil
}

// reads file at userHomeDir/.gatorconfig.json and outputs Config struct
func Read() Config {
	//get file path
	path, err := getFilePath()
	check(err)
	//initialize Config struct
	config := Config{}

	//read the config file into []bytes
	data, err := os.ReadFile(path)
	check(err)

	//unmarshal the json into the config struct
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
