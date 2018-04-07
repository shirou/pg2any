package main

import (
	"database/sql"
)

type Sphinx struct {
	db         *sql.DB
	output     string
	templates  string
	overwrites []string
	config     GeneratorConfig
}

const SphinxTypeName = "sphinx"

/*
func NewSphinx(db *sql.DB, config GeneratorConfig) (Generator, error) {
	ret := Sphinx{
		db:         db,
		templates:  config.Templates,
		output:     config.Output,
		overwrites: config.Overwrites,
		config:     config,
	}

	return &ret, nil
}

func (gen *Sphinx) GetType() string {
	return SphinxTypeName
}
func (gen *Sphinx) LoadTemplates() error {
	return nil
}

func (gen *Sphinx) Output(ins InspectResult) error {
	log.Printf("output: %s", gen.output)
	log.Printf("templates: %s", gen.templates)

	if err := DirExists(gen.output); err != nil {
		return err
	}

	return nil
}
*/
