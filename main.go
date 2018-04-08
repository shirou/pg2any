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

func DirExists(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("output dir %s is not exists", dir)
		}
	}
	if info.IsDir() == false {
		return fmt.Errorf("output dir %s is not directory", dir)
	}
	return nil
}

func SnakeToUpperCamel(src string) string {
	var ret []string
	for _, b := range strings.Split(src, "_") {
		ret = append(ret, strings.Title(b))
	}
	return strings.Join(ret, "")
}

func SnakeToLowerCamel(src string) string {
	var ret []string
	for i, b := range strings.Split(src, "_") {
		if i == 0 {
			ret = append(ret, strings.ToLower(b))
		} else {
			ret = append(ret, strings.Title(b))
		}
	}
	return strings.Join(ret, "")
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
