package main

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

const confFile = "conf.yaml"

type Config struct {
	Listen string `yaml:"listen"`
	Server string `yaml:"server"`
}

var ConfigParam Config = Config{"", ""}

func checkConfFile() bool {
	if _, err := os.Stat(confFile); os.IsNotExist(err) {
		LOGE("conf.yaml does not exist")
		return false
	} else {
		LOGI("conf.yaml exists")
		return true
	}
}

func loadConf() bool {
	data, err := ioutil.ReadFile("conf.yaml")
	if err != nil {
		LOGE(err)
		return false
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		LOGE(err)
		return false
	} else {
		LOGI("config: ", config)
		ConfigParam = config
		return true
	}
}

func initConf() bool {
	if checkConfFile() == false {
		return false
	}

	if loadConf() == false {
		return false
	}
	return true
}
