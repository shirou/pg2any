package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"
)

type SphinxConfig struct {
	Output       string   `json:"output"`
	Templates    string   `json:"templates"`
	IgnoreTables []string `json:"ignore_tables"`
}

type Sphinx struct {
	db       *sql.DB
	config   SphinxConfig
	ins      InspectResult
	template *template.Template
	root     string
}

type SphinxMember struct {
	Name       string
	Type       string
	Constraint string
	Comment    string
}

type SphinxTypeMember struct {
	Name    string
	Comment string
	Values  []string
}

const SphinxTypeName = "sphinx"

func NewSphinx(db *sql.DB, root string, raw json.RawMessage) (Generator, error) {
	config, err := loadSphinxConfig(root, raw)
	if err != nil {
		return nil, err
	}
	ret := Sphinx{
		db:     db,
		config: config,
		root:   root,
	}

	return &ret, nil
}

func (gen *Sphinx) GetType() string {
	return SphinxTypeName
}

func (gen *Sphinx) Build(ins InspectResult) error {
	log.Printf("output: %s", filePathJoinRoot(gen.root, gen.config.Output))
	log.Printf("templates: %s", filePathJoinRoot(gen.root, gen.config.Templates))
	gen.ins = ins

	// Load templates
	funcs := template.FuncMap{
		"writeUnderLine": func(s, char string) string { return strings.Repeat(char, len(s)) },
	}
	tdir := filepath.Join(filePathJoinRoot(gen.root, gen.config.Templates), "*.tmpl")
	t := template.Must(template.New("").Funcs(funcs).ParseGlob(tdir))

	gen.template = t

	// Build tables
	for _, table := range gen.ins.Tables {
		if contains(gen.config.IgnoreTables, table.Name) {
			continue
		}
		fileName := SnakeToUpperCamel(table.Name) + ".rst"
		file, err := os.Create(filepath.Join(filePathJoinRoot(gen.root, gen.config.Output), fileName))
		defer file.Close()
		if err != nil {
			return errors.Wrap(err, "build create file")
		}
		if err := gen.buildTable(file, table); err != nil {
			return errors.Wrap(err, "build write table")
		}
	}

	// Build types
	enumFileName := "enum.rst"
	file, err := os.Create(filepath.Join(filePathJoinRoot(gen.root, gen.config.Output), enumFileName))
	defer file.Close()
	if err != nil {
		return errors.Wrap(err, "build create file")
	}
	if err := gen.buildType(file, gen.ins.Types); err != nil {
		return errors.Wrap(err, "build write type")
	}

	return nil
}

func (gen *Sphinx) buildTable(wr io.Writer, table Table) error {
	return gen.template.ExecuteTemplate(wr, "table", map[string]interface{}{
		"now":     time.Now().UTC().Format(time.RFC3339),
		"comment": table.Comment.String,
		"name":    table.Name,
		"member":  gen.members(table),
	})
}

func (gen *Sphinx) members(table Table) []SphinxMember {
	var ret []SphinxMember

	for _, col := range table.Columns {
		var cons string
		switch col.Constraint.String {
		case "p":
			cons = "Primary"
		case "f":
			cons = col.ConstraintSrc.String
		case "c":
			cons = col.ConstraintSrc.String
		}
		dtype := col.DataType
		if col.Serial {
			dtype += "(serial)"
		}

		m := SphinxMember{
			Name:       col.Name,
			Type:       dtype,
			Constraint: cons,
			Comment:    strings.Replace(col.Comment.String, "\n", "", -1),
		}
		ret = append(ret, m)
	}
	return ret
}

func (gen *Sphinx) buildType(wr io.Writer, types []Type) error {
	var members []SphinxTypeMember
	for _, typ := range types {
		var vs []string
		for _, val := range typ.Values {
			vs = append(vs, val)
		}
		m := SphinxTypeMember{
			Name:    typ.Name,
			Comment: typ.Comment.String,
			Values:  vs,
		}
		members = append(members, m)
	}

	return gen.template.ExecuteTemplate(wr, "enum", map[string]interface{}{
		"now":     time.Now().UTC().Format(time.RFC3339),
		"members": members,
	})
	return nil
}

func loadSphinxConfig(root string, raw json.RawMessage) (SphinxConfig, error) {
	var pbc SphinxConfig
	if err := json.Unmarshal(raw, &pbc); err != nil {
		return pbc, fmt.Errorf("protobuf config error: %s", err)
	}
	output := filePathJoinRoot(root, pbc.Output)
	if err := DirExists(output); err != nil {
		return pbc, fmt.Errorf("protobuf output is not exists: %s", pbc.Output)
	}
	return pbc, nil
}

func writeUnderLine(s string, char string) string {
	return strings.Repeat(char, len(s))
}
