package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Generator interface {
	GetType() string
	Build(InspectResult) error
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

func SnakeToUpper(src string) string {
	var ret []string
	for _, b := range strings.Split(src, "_") {
		ret = append(ret, strings.ToUpper(b))
	}
	return strings.Join(ret, "_")
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

func isNumber(v string) bool {
	if _, err := strconv.Atoi(v); err == nil {
		return true
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return true
	}
	return false
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func partContains(s []string, target string) bool {
	for _, a := range s {
		if strings.Contains(target, a) {
			return true
		}
	}
	return false
}

func filePathJoinRoot(root, file string) string {
	if filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(root, file)
}
