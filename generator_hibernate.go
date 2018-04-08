package main

import (
	"bytes"
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

type HibernateConfig struct {
	Output          string   `json:"output"`
	Templates       string   `json:"templates"`
	Overwrites      []string `json:"overwrites"`
	PackageName     string   `json:"package_name"`
	IgnoreTables    []string `json:"ignore_tables"`
	ReadOnlyColumns []string `json:"read_only_columns"`
	IgnoreColumns   []string `json:"ignore_columns"`
}

type Hibernate struct {
	db       *sql.DB
	config   HibernateConfig
	ins      InspectResult
	template *template.Template
	root     string
}

type HibernateMember struct {
	Name    string
	Type    string
	Comment string
}

type HibernateAccessor struct {
	get  bool
	name string
	typ  string
}

const HibernateTypeName = "hibernate"

func NewHibernate(db *sql.DB, root string, raw json.RawMessage) (Generator, error) {
	config, err := loadHibernateConfig(root, raw)
	if err != nil {
		return nil, err
	}
	ret := Hibernate{
		db:     db,
		config: config,
		root:   root,
	}

	return &ret, nil
}

func (gen *Hibernate) GetType() string {
	return HibernateTypeName
}

func (gen *Hibernate) Build(ins InspectResult) error {
	log.Printf("output: %s", filepath.Join(gen.root, gen.config.Output))
	log.Printf("templates: %s", filepath.Join(gen.root, gen.config.Templates))
	gen.ins = ins

	// Load templates
	tdir := filepath.Join(gen.root, gen.config.Templates, "*.tmpl")
	t := template.Must(template.ParseGlob(tdir))
	gen.template = t

	// Build tables
	for _, table := range gen.ins.Tables {
		if contains(gen.config.IgnoreTables, table.Name) {
			continue
		}

		fileName := SnakeToUpperCamel(table.Name) + ".java"

		file, err := os.Create(filepath.Join(gen.root, gen.config.Output, fileName))
		defer file.Close()
		if err != nil {
			return errors.Wrap(err, "build create file")
		}
		if err := gen.buildTable(file, table); err != nil {
			return errors.Wrap(err, "build write table")
		}
	}

	// Build types
	for _, typ := range gen.ins.Types {
		fileName := SnakeToUpperCamel(typ.Name) + ".java"
		file, err := os.Create(filepath.Join(gen.root, gen.config.Output, fileName))
		defer file.Close()
		if err != nil {
			return errors.Wrap(err, "build create file")
		}
		if err := gen.buildType(file, typ); err != nil {
			return errors.Wrap(err, "build write type")
		}
	}

	return nil
}

func (gen *Hibernate) buildTable(wr io.Writer, table Table) error {
	return gen.template.ExecuteTemplate(wr, "class", map[string]interface{}{
		"package_name": gen.config.PackageName,
		"now":          time.Now().UTC().Format(time.RFC3339),
		"table":        table,
		"name":         SnakeToUpperCamel(table.Name),
		"member":       gen.members(table),
		"accessor":     gen.accessor(table),
	})
}

func (gen *Hibernate) members(table Table) []HibernateMember {
	var ret []HibernateMember

	for _, col := range table.Columns {
		m := HibernateMember{
			Name:    SnakeToLowerCamel(col.Name),
			Type:    gen.convertType(col),
			Comment: col.Comment.String,
		}
		ret = append(ret, m)
	}
	return ret
}

func (gen *Hibernate) accessor(table Table) []string {
	var ret []string

	for _, col := range table.Columns {
		getter, err := gen.getter(col)
		if err != nil {
			log.Fatal(err)
		}
		ret = append(ret, getter)
		if contains(gen.config.ReadOnlyColumns, col.Name) {
			continue
		}

		setter, err := gen.setter(col)
		if err != nil {
			log.Fatal(err)
		}
		ret = append(ret, setter)
	}
	return ret
}

func (gen *Hibernate) getter(col Column) (string, error) {
	var ret bytes.Buffer
	data := map[string]interface{}{
		"func":       SnakeToUpperCamel(col.Name),
		"name":       SnakeToLowerCamel(col.Name),
		"type":       gen.convertType(col),
		"anotations": gen.anotations(col),
	}
	if err := gen.template.ExecuteTemplate(&ret, "getter", data); err != nil {
		return "", errors.Wrap(err, "getter: "+col.Name)
	}

	return ret.String(), nil
}

func (gen *Hibernate) anotations(col Column) []string {
	var ret []string
	var unique bool
	if col.Constraint.String == "p" {
		ret = append(ret, "@Id")
		unique = true
	}
	if col.Constraint.String == "u" {
		unique = true
	}
	if gen.enumExists(col.DataType) {
		ret = append(ret, "@Enumerated(EnumType.STRING)")
	}

	ret = append(ret, fmt.Sprintf(`@Column(name="%s", unique=%t, nullable=%t)`, col.Name, unique, !col.NotNull))

	return ret
}

func (gen *Hibernate) setter(col Column) (string, error) {
	var ret bytes.Buffer
	var constraint string
	if col.Constraint.String == "c" {
		constraint = "    // " + col.ConstraintSrc.String
	}

	data := map[string]interface{}{
		"func":       SnakeToUpperCamel(col.Name),
		"name":       SnakeToLowerCamel(col.Name),
		"type":       gen.convertType(col),
		"constraint": constraint,
	}
	if err := gen.template.ExecuteTemplate(&ret, "setter", data); err != nil {
		return "", errors.Wrap(err, "setter: "+col.Name)
	}

	return ret.String(), nil
}

func (gen *Hibernate) buildType(wr io.Writer, typ Type) error {
	members := strings.Join(typ.Values, ", ") + ";"

	return gen.template.ExecuteTemplate(wr, "enum", map[string]interface{}{
		"package_name": gen.config.PackageName,
		"now":          time.Now().UTC().Format(time.RFC3339),
		"name":         SnakeToUpperCamel(typ.Name),
		"type":         typ,
		"members":      members,
	})
	return nil
}

func (gen *Hibernate) enumExists(typeName string) bool {
	for _, typ := range gen.ins.Types {
		if typ.Name == typeName {
			return true
		}
	}
	return false
}

func (gen *Hibernate) convertType(col Column) string {
	// http://docs.jboss.org/hibernate/orm/5.2/userguide/html_single/Hibernate_User_Guide.html#basic

	switch col.DataType {
	case "text":
		return "String"
	case "int":
		return "Integer"
	case "float":
		return "Float"
	case "double":
		return "double"
	case "bigint":
		return "long"
	case "uuid":
		return "UUID"
	case "numeric":
		return "BigDecimal"
	case "timestamp", "date", "timestamp with time zone":
		return "Date"
	case "boolean":
		return "boolean"
	default:
		typ, err := gen.ins.FindType(col.DataType)
		if err == nil {
			return SnakeToUpperCamel(typ.Name)
		}
	}
	return col.DataType
}

func loadHibernateConfig(root string, raw json.RawMessage) (HibernateConfig, error) {
	var hc HibernateConfig
	if err := json.Unmarshal(raw, &hc); err != nil {
		return hc, fmt.Errorf("hibernate config error: %s", err)
	}
	output := filepath.Join(root, hc.Output)
	if err := DirExists(output); err != nil {
		return hc, fmt.Errorf("hibernate output is not exists: %s", hc.Output)
	}
	return hc, nil
}
