package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var confFile string
	flag.StringVar(&confFile, "c", "", "config file path")
	flag.Parse()
	if confFile == "" {
		path, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		path = filepath.Dir(path)
		c, err := searchConfigFile(path)
		if err != nil {
			log.Fatal(fmt.Errorf("could not find config file on " + path))
		}
		confFile = c
	}

	config, err := NewConfig(confFile)
	if err != nil {
		log.Fatal(fmt.Errorf("config file error: %s", err))
	}

	ins, err := Inspect(config.db)
	if err != nil {
		log.Fatal(err)
	}

	for _, gen := range config.generators {
		log.Printf("Generate: %s", gen.GetType())
		if err := gen.Build(ins); err != nil {
			log.Fatal(err)
		}
		log.Printf("done")
	}
}

func searchConfigFile(dir string) (string, error) {
	glob := filepath.Join(dir, "*.json")

	files, err := filepath.Glob(glob)
	if err != nil {
		log.Fatal(err)
	}
	if files == nil {
		return "", fmt.Errorf("no matching json file: " + dir)
	}

	for _, file := range files {
		b, err := ioutil.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}
		s := string(b)
		if strings.Contains(s, "generators") &&
			strings.Contains(s, "output") &&
			strings.Contains(s, "templates") {
			return file, nil
		}
	}

	return "", fmt.Errorf("no matching json file: " + dir)
}
