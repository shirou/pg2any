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
	"regexp"
	"strings"
	"text/template"
	"time"
	"unicode"

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
	GenerateMetamodel    bool     `json:"generate_metamodel"`
	VersionFieldColumn   string   `json:"version_field_column"`
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

type HibernateMetamodel struct {
	Attr    string
	ClsName string
	Name    string
	Type    string
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
		if partContains(gen.config.IgnoreTables, table.Name) {
			continue
		}

		fileName := SnakeToUpperCamel(table.Name) + ".java"
		file, err := os.Create(filepath.Join(filePathJoinRoot(gen.root, gen.config.Output), fileName))
		if err != nil {
			return errors.Wrap(err, "build create file")
		}
		if err := gen.buildTable(file, table); err != nil {
			file.Close()
			return errors.Wrap(err, "build write table")
		}

		if gen.config.GenerateMetamodel {
			// generate meta model class file
			metaFileName := SnakeToUpperCamel(table.Name) + "_.java"
			metaFile, err := os.Create(filepath.Join(filePathJoinRoot(gen.root, gen.config.Output), metaFileName))
			if err != nil {
				file.Close()
				return errors.Wrap(err, "create metamodel file")
			}
			if err := gen.buildMetamodel(metaFile, table); err != nil {
				file.Close()
				metaFile.Close()
				return errors.Wrap(err, "build write metamodel")
			}
			metaFile.Close()
		}
		file.Close()
	}

	// Build types
	for _, typ := range gen.ins.Types {
		fileName := SnakeToUpperCamel(typ.Name) + ".java"
		file, err := os.Create(filepath.Join(filePathJoinRoot(gen.root, gen.config.Output), fileName))
		if err != nil {
			return errors.Wrap(err, "build create file")
		}

		utFileName := SnakeToUpperCamel(typ.Name) + "UserType.java"
		utFile, err := os.Create(filepath.Join(filePathJoinRoot(gen.root, gen.config.Output), utFileName))
		if err != nil {
			file.Close()
			return errors.Wrap(err, "build usertype file")
		}

		if err := gen.buildType(file, utFile, typ); err != nil {
			file.Close()
			utFile.Close()
			return errors.Wrap(err, "build write type")
		}
		file.Close()
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

func (gen *Hibernate) buildMetamodel(wr io.Writer, table Table) error {
	return gen.template.ExecuteTemplate(wr, "metamodel", map[string]interface{}{
		"package_name": gen.config.PackageName,
		"name":         SnakeToUpperCamel(table.Name),
		"member":       gen.metamodel(table),
	})
}

func (gen *Hibernate) members(table Table) []HibernateMember {
	var ret []HibernateMember
	hasPrimary := false

	for _, col := range table.Columns {
		t := gen.convertType(col)
		if col.Array {
			t = fmt.Sprintf("%s[]", t)
		}
		if col.PrimaryKey {
			hasPrimary = true
		}

		m := HibernateMember{
			Name:    SnakeToLowerCamel(col.Name),
			Type:    t,
			Comment: strings.Replace(col.Comment.String, "\n", "", -1),
		}
		ret = append(ret, m)
	}
	if !hasPrimary {
		log.Printf("WARN: %s doesn't has primary key", table.Name)
	}

	return ret
}

func (gen *Hibernate) metamodel(table Table) []HibernateMetamodel {
	var ret []HibernateMetamodel
	for _, col := range table.Columns {
		t := gen.convertType(col)
		attr := "SingularAttribute" // Only Singular is used

		typ := strings.Title(t)
		if col.Array {
			typ = typ + "[]"
		}

		m := HibernateMetamodel{
			Attr:    attr,
			ClsName: SnakeToUpperCamel(table.Name),
			Name:    decapitalize(SnakeToUpperCamel(col.Name)),
			Type:    typ,
		}
		ret = append(ret, m)
	}
	return ret
}

// decapitalize is a utility method to convert to normal Java variable capitalization.
// This measns the first charactor from upper case to lower case. However, this has a special case,
// there is more than one character and both the first and  second characters are upper case, we leave it alone.
// https://github.com/hibernate/hibernate-orm/blob/e5dc635a52362f69b69acb8d5b166b69b165dbbd/tooling/metamodel-generator/src/main/java/org/hibernate/jpamodelgen/util/StringUtil.java#L87
func decapitalize(name string) string {
	if name == "" || startsWithSeveralUpperCaseLetters(name) || len(name) < 2 {
		return name
	} else {
		return strings.ToLower(string(name[0])) + strings.ToLower(string(name[1])) + string(name[2:])

	}
}

func startsWithSeveralUpperCaseLetters(str string) bool {
	return len(str) > 1 && unicode.IsUpper(rune(str[0])) && unicode.IsUpper(rune(str[1]))
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
	t := gen.convertType(col)
	if col.Array {
		t = fmt.Sprintf("%s[]", t)
	}
	data := map[string]interface{}{
		"func":       SnakeToUpperCamel(col.Name),
		"name":       SnakeToLowerCamel(col.Name),
		"type":       t,
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

var regNextval = regexp.MustCompile(`^nextval\('.+_seq'::regclass\)`)

func isSequence(col Column) bool {
	if col.PrimaryKey && regNextval.MatchString(col.DefaultValue.String) {
		return true
	}
	return false
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
	if col.Serial || isSequence(col) {
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

	if col.Array {
		t := strings.Title(gen.convertType(col))
		ret = append(ret, fmt.Sprintf(`@Type(type = "%sArrayUserType")`, t))
	}

	if gen.config.VersionFieldColumn == col.Name {
		ret = append(ret, fmt.Sprintf("@javax.persistence.Version"))
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
	if contains(gen.config.NotInsertableColumns, col.Name) && contains(gen.config.NotUpdatableColumns, col.Name) {
		scope = "private"
	}

	t := gen.convertType(col)
	if col.Array {
		t = fmt.Sprintf("%s[]", t)
	}
	data := map[string]interface{}{
		"func":       SnakeToUpperCamel(col.Name),
		"name":       SnakeToLowerCamel(col.Name),
		"type":       t,
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

	// Remove array charactor (we can still get this column is array from other value)
	t := strings.Replace(col.DataType, "[]", "", 1)

	// http://docs.jboss.org/hibernate/orm/5.2/userguide/html_single/Hibernate_User_Guide.html#basic

	switch t {
	case "text":
		return "String"
	case "int", "integer":
		return "Integer"
	case "float":
		return "Float"
	case "double", "double precision":
		return "Double"
	case "bigint":
		return "Long"
	case "serial":
		return "Integer"
	case "bigserial":
		return "Long"
	case "uuid":
		return "UUID"
	case "bytea":
		return "byte[]" // always byte[]
	case "numeric":
		return "BigDecimal"
	case "date":
		return "LocalDate"
	case "json", "jsonb":
		return "JsonObject"
	case "timestamp":
		return "Timestamp"
	case "boolean":
		return "boolean"
	default:
		// "timestamp with time zone", "timestamp without time zone", "timestamp(n) with time zone"
		if strings.HasSuffix(t, "time zone") {
			return "OffsetDateTime"
		}
		if strings.HasPrefix(t, "numeric") {
			return "BigDecimal"
		}
		if strings.HasPrefix(t, "character") {
			return "String"
		}

		typ, err := gen.ins.FindType(t)
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
	output := filePathJoinRoot(root, hc.Output)
	if err := DirExists(output); err != nil {
		return hc, fmt.Errorf("hibernate output is not exists: %s", hc.Output)
	}
	return hc, nil
}
