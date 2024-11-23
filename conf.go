package main

import (
	"fmt"
	"os"
		"gopkg.in/yaml.v2"
		"io/ioutil"
		"log"
	)

func checkConf() {
	if _, err := os.Stat("conf.yaml"); os.IsNotExist(err) {
		fmt.Println("conf.yaml does not exist")
		return false
	} else {
		fmt.Println("conf.yaml exists")
		return true
	}
}

type Config struct {
	Listen string `yaml:"listen"`
	Server string `yaml:"server"`
}

func readConfig() (*Config, error) {
	data, err := ioutil.ReadFile("conf.yaml")
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func main() {
	if _, err := os.Stat("conf.yaml"); os.IsNotExist(err) {
		fmt.Println("conf.yaml does not exist")
	} else {
		fmt.Println("conf.yaml exists")
		config, err := readConfig()
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Printf("Listen: %s, Server: %s\n", config.Listen, config.Server)
	}
}