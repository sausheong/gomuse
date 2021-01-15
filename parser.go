package main

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

type Music struct {
	Key    string  `yaml:"key"`
	Length float64 `yaml:"length"`
	Notes  []struct {
		C1 []string `yaml:"C1"`
		C2 []string `yaml:"C2"`
	} `yaml:"notes"`
}

// Parse reads a muse notation file and converts into a Tune
func Parse(m *Music, name string) (err error) {
	yamlFile, err := ioutil.ReadFile(name)
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, m)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}
	return
}
