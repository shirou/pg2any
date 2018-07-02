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
	Output               string   `json:"output"`
	Templates            string   `json:"templates"`
	Overwrites           []string `json:"overwrites"`
	PackageName          string   `json:"package_name"`
	IgnoreTables         []string `json:"ignore_tables"`
	NotInsertableColumns []string `json:"not_insertable_columns"`
	NotUpdatableColumns  []string `json:"not_updatable_columns"`
	IgnoreColumns        []string `json:"ignore_columns"`
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
	log.Printf("output: %s", filePathJoinRoot(gen.root, gen.config.Output))
	log.Printf("templates: %s", filePathJoinRoot(gen.root, gen.config.Templates))
	gen.ins = ins

	// Load templates
	tdir := filepath.Join(filePathJoinRoot(gen.root, gen.config.Templates), "*.tmpl")
	t := template.Must(template.ParseGlob(tdir))
	gen.template = t

	// Build tables
	for _, table := range gen.ins.Tables {
		if contains(gen.config.IgnoreTables, table.Name) {
			continue
		}

		fileName := SnakeToUpperCamel(table.Name) + ".java"

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
	for _, typ := range gen.ins.Types {
		fileName := SnakeToUpperCamel(typ.Name) + ".java"
		file, err := os.Create(filepath.Join(filePathJoinRoot(gen.root, gen.config.Output), fileName))
		defer file.Close()
		if err != nil {
			return errors.Wrap(err, "build create file")
		}

		utFileName := SnakeToUpperCamel(typ.Name) + "UserType.java"
		utFile, err := os.Create(filepath.Join(filePathJoinRoot(gen.root, gen.config.Output), utFileName))
		defer utFile.Close()
		if err != nil {
			return errors.Wrap(err, "build usertype file")
		}

		if err := gen.buildType(file, utFile, typ); err != nil {
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
			Comment: strings.Replace(col.Comment.String, "\n", "", -1),
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

func parseForignTable(src string) (string, string) {
	// FOREIGN KEY (security_code) REFERENCES master_security(security_code)
	return "", ""
}

func (gen *Hibernate) anotations(col Column) []string {
	var ret []string
	if col.PrimaryKey {
		ret = append(ret, "@Id")
	}
	if col.Unique {
		ret = append(ret, "@UniqueConstraint")
	}
	if col.ForignTable.Valid {
		// a := `@JoinColumns({ @JoinColumn(name="userid", referencedColumnName="id") })`
		// ret = append(ret, "// ForignTable = "+col.ForignTable.String)
	}
	if col.Serial {
		ret = append(ret, "@GeneratedValue(strategy=GenerationType.IDENTITY)")
	}

	if gen.enumExists(col.DataType) {
		ret = append(ret, fmt.Sprintf(`@Type(type = "%s.%sUserType")`,
			gen.config.PackageName,
			SnakeToUpperCamel(col.DataType)))
	}

	if col.DataType == "json" || col.DataType == "jsonb" {
		ret = append(ret, `@Type(type = "JsonUserType")`)
	}

	column_args := make([]string, 0)
	column_args = append(column_args, fmt.Sprintf(`name="%s"`, col.Name))
	column_args = append(column_args, fmt.Sprintf("nullable=%t", !col.NotNull))
	if contains(gen.config.NotInsertableColumns, col.Name) {
		column_args = append(column_args, "insertable=false")
	}
	if contains(gen.config.NotUpdatableColumns, col.Name) {
		column_args = append(column_args, "updatable=false")
	}

	ret = append(ret, fmt.Sprintf(`@Column(%s)`, strings.Join(column_args, ", ")))

	return ret
}

func (gen *Hibernate) setter(col Column) (string, error) {
	var ret bytes.Buffer
	var constraint string
	if col.Constraint.String == "c" {
		constraint = "    // " + col.ConstraintSrc.String
	}

	var scope = "public"
	if contains(gen.config.NotInsertableColumns, col.Name) {
		scope = "private"
	}
	if contains(gen.config.NotUpdatableColumns, col.Name) {
		scope = "private"
	}

	data := map[string]interface{}{
		"func":       SnakeToUpperCamel(col.Name),
		"name":       SnakeToLowerCamel(col.Name),
		"type":       gen.convertType(col),
		"scope":      scope,
		"constraint": constraint,
	}
	if err := gen.template.ExecuteTemplate(&ret, "setter", data); err != nil {
		return "", errors.Wrap(err, "setter: "+col.Name)
	}

	return ret.String(), nil
}

func (gen *Hibernate) buildType(wr, utwr io.Writer, typ Type) error {
	var mem []string
	dt := "String"

	for _, val := range typ.Values {
		if isNumber(val) {
			mem = append(mem, fmt.Sprintf("VALUE_%s(%s)", SnakeToUpper(val), val))
			dt = "Integer"
		} else {
			mem = append(mem, fmt.Sprintf(`%s("%s")`, SnakeToUpper(val), val))
		}
	}

	members := strings.Join(mem, ", ") + ";"

	if err := gen.template.ExecuteTemplate(wr, "enum", map[string]interface{}{
		"package_name": gen.config.PackageName,
		"now":          time.Now().UTC().Format(time.RFC3339),
		"name":         SnakeToUpperCamel(typ.Name),
		"type":         typ,
		"dt":           dt,
		"members":      members,
	}); err != nil {
		return err
	}

	if err := gen.template.ExecuteTemplate(utwr, "enum_usertype", map[string]interface{}{
		"package_name": gen.config.PackageName,
		"now":          time.Now().UTC().Format(time.RFC3339),
		"name":         SnakeToUpperCamel(typ.Name),
		"snake":        (typ.Name),
		"type":         typ,
		"dt":           dt,
		"members":      members,
	}); err != nil {
		return err
	}

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
	// numeric with presidion is double
	if strings.Contains(col.DataType, "numeric(") {
		return "BigDecimal"
	}

	// http://docs.jboss.org/hibernate/orm/5.2/userguide/html_single/Hibernate_User_Guide.html#basic

	array := false
	if strings.HasSuffix(col.DataType, "[]") {
		array = true
		col.DataType = strings.Replace(col.DataType, "[]", "", 1)
	}

	f := func(t string, a bool) string {
		if a {
			return fmt.Sprintf("ArrayList<%s>", t)
		}
		return t
	}

	switch col.DataType {
	case "text":
		return f("String", array)
	case "int", "integer":
		return f("Integer", array)
	case "float":
		return f("Float", array)
	case "double":
		return f("double", array)
	case "bigint":
		return f("long", array)
	case "serial":
		return f("Integer", array)
	case "bigserial":
		return f("long", array)
	case "uuid":
		return f("UUID", array)
	case "bytea":
		return "byte[]" // always byte[]
	case "numeric":
		return f("BigDecimal", array)
	case "date":
		return f("Date", array)
	case "json", "jsonb":
		return f("Map<String, String>", array)
	case "timestamp":
		return f("Timestamp", array)
	case "timestamp with time zone", "timestamp without time zone":
		return f("OffsetDateTime", array)
	case "boolean":
		return f("boolean", array)
	default:
		if strings.HasPrefix(col.DataType, "numeric") {
			return f("BigDecimal", array)
		}
		if strings.HasPrefix(col.DataType, "character") {
			return f("String", array)
		}

		typ, err := gen.ins.FindType(col.DataType)
		if err == nil {
			return f(SnakeToUpperCamel(typ.Name), array)
		}
	}
	return f(col.DataType, array)
}

func loadHibernateConfig(root string, raw json.RawMessage) (HibernateConfig, error) {
	var hc HibernateConfig
	if err := json.Unmarshal(raw, &hc); err != nil {
		return hc, fmt.Errorf("hibernate config error: %s", err)
	}
	output := filePathJoinRoot(root, hc.Output)
	if err := DirExists(output); err != nil {
		return hc, fmt.Errorf("hibernate output is not exists: %s", hc.Output)
	}
	return hc, nil
}
