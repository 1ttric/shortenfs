package config

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

// Defines the config for the currently used block filesystem
type ShortenBlockConfig struct {
	// Drive name to use for this filesystem
	Driver string
	// Root ID for this filesystem
	RootID string
	// Depth of the node tree for this filesystem
	Depth int
	// Driver-specific options (defined in each driver)
	DriverOpts interface{}
}

var (
	lastCfgFile string
	MainConfig  ShortenBlockConfig
)

func Read(cfgFile string) {
	log.Infof("using config file %s", cfgFile)
	lastCfgFile = cfgFile
	data, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Fatalf("could not read config file: %s", err.Error())
	}
	err = yaml.Unmarshal(data, &MainConfig)
	if err != nil {
		log.Fatalf("could not unmarshal config file: %s", err.Error())
	}
}

func Write() {
	log.Infof("saving config to file %s", lastCfgFile)
	data, err := yaml.Marshal(MainConfig)
	if err != nil {
		log.Fatalf("could not marshal config file: %s", err.Error())
	}
	err = ioutil.WriteFile(lastCfgFile, data, 0o644)
	if err != nil {
		log.Fatalf("could not write config file: %s", err.Error())
	}
}
