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

type ProtoBufConfig struct {
	Output             string   `json:"output"`
	Templates          string   `json:"templates"`
	Overwrites         []string `json:"overwrites"`
	PackageName        string   `json:"package_name"`
	EnumDir            string   `json:"enum_dir"`
	JavaPackage        string   `json:"java_package"`
	GoPackage          string   `json:"go_package"`
	IgnoreTables       []string `json:"ignore_tables"`
	UseStringToNumeric bool     `json:"use_string_to_numeric"`
}

type ProtoBuf struct {
	db       *sql.DB
	config   ProtoBufConfig
	ins      InspectResult
	template *template.Template
	root     string
}

type ProtoBufMember struct {
	Constraint string
	Name       string
	Type       string
	Comment    string
	Index      int
}

type ProtoBufTypeMember struct {
	Name    string
	Comment string
	Values  string
}

const ProtoBufTypeName = "protobuf"

func NewProtoBuf(db *sql.DB, root string, raw json.RawMessage) (Generator, error) {
	config, err := loadProtoBufConfig(root, raw)
	if err != nil {
		return nil, err
	}
	ret := ProtoBuf{
		db:     db,
		config: config,
		root:   root,
	}

	return &ret, nil
}

func (gen *ProtoBuf) GetType() string {
	return ProtoBufTypeName
}

func (gen *ProtoBuf) Build(ins InspectResult) error {
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
		fileName := SnakeToUpperCamel(table.Name) + "Message.proto"
		file, err := os.Create(filepath.Join(filePathJoinRoot(gen.root, gen.config.Output), fileName))
		if err != nil {
			return errors.Wrap(err, "build create file")
		}
		if err := gen.buildTable(file, table); err != nil {
			file.Close()
			return errors.Wrap(err, "build write table")
		}
		file.Close()
	}

	// Build types
	enumFileName := "enum.proto"
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

func (gen *ProtoBuf) buildTable(wr io.Writer, table Table) error {
	return gen.template.ExecuteTemplate(wr, "message", map[string]interface{}{
		"package_name": gen.config.PackageName,
		"java_package": gen.config.JavaPackage,
		"go_package":   gen.config.GoPackage,
		"now":          time.Now().UTC().Format(time.RFC3339),
		"comment":      table.Comment.String,
		"table":        table,
		"name":         SnakeToUpperCamel(table.Name) + "Message",
		"member":       gen.members(table),
		"enum_path":    filepath.Join(gen.config.EnumDir, "enum.proto"),
	})
}

func (gen *ProtoBuf) members(table Table) []ProtoBufMember {
	var ret []ProtoBufMember

	for i, col := range table.Columns {
		m := ProtoBufMember{
			Name:    col.Name,
			Type:    gen.convertType(col),
			Comment: strings.Replace(col.Comment.String, "\n", "", -1),
			Index:   i + 1,
		}
		ret = append(ret, m)
	}
	return ret
}

func (gen *ProtoBuf) buildType(wr io.Writer, types []Type) error {
	var members []ProtoBufTypeMember
	for _, typ := range types {
		name := SnakeToUpper(typ.Name)
		var vs []string
		for i, val := range typ.Values {
			if isNumber(val) {
				vs = append(vs, fmt.Sprintf("%s_VALUE_%s = %d;", name, SnakeToUpper(val), i))
			} else {
				vs = append(vs, fmt.Sprintf("%s_%s = %d;", name, SnakeToUpper(val), i))
			}
		}
		m := ProtoBufTypeMember{
			Name:    SnakeToUpperCamel(typ.Name),
			Comment: typ.Comment.String,
			Values:  "  " + strings.Join(vs, "\n  "),
		}
		members = append(members, m)
	}

	return gen.template.ExecuteTemplate(wr, "enum", map[string]interface{}{
		"package_name": gen.config.PackageName,
		"java_package": gen.config.JavaPackage,
		"go_package":   gen.config.GoPackage,
		"now":          time.Now().UTC().Format(time.RFC3339),
		"members":      members,
	})
	return nil
}

func (gen *ProtoBuf) enumExists(typeName string) bool {
	for _, typ := range gen.ins.Types {
		if typ.Name == typeName {
			return true
		}
	}
	return false
}

func (gen *ProtoBuf) convertType(col Column) string {
	// https://developers.google.com/protocol-buffers/docs/proto3#simple

	var array = ""
	if strings.HasSuffix(col.DataType, "[]") {
		array = "repeated "
		col.DataType = strings.Replace(col.DataType, "[]", "", 1)
	}

	switch col.DataType {
	case "text":
		return array + "string"
	case "int", "integer":
		return array + "int32"
	case "float":
		return array + "float"
	case "double", "double precision":
		return array + "double"
	case "bigint":
		return array + "int64"
	case "serial":
		return array + "int32"
	case "bigserial":
		return array + "int64"
	case "uuid":
		return array + "string"
	case "bytea":
		return array + "bytes"
	case "numeric":
		if gen.config.UseStringToNumeric {
			return array + "string"
		}
		return array + "int64"
	case "date":
		return array + "string"
	case "boolean":
		return array + "bool"
	case "json", "jsonb":
		return array + "map<string, string>"
	default:
		// "timestamp with time zone", "timestamp without time zone", "timestamp(n) with time zone"
		if strings.HasSuffix(col.DataType, "time zone") {
			return array + "google.protobuf.Timestamp"
		}
		if strings.HasPrefix(col.DataType, "numeric") {
			if gen.config.UseStringToNumeric {
				return array + "string"
			}
			return array + "int64"
		}
		if strings.HasPrefix(col.DataType, "character") {
			return array + "string"
		}

		typ, err := gen.ins.FindType(col.DataType)
		if err == nil {
			return array + gen.config.PackageName + "." + SnakeToUpperCamel(typ.Name)
		}
	}
	return array + col.DataType
}

func loadProtoBufConfig(root string, raw json.RawMessage) (ProtoBufConfig, error) {
	var pbc ProtoBufConfig
	if err := json.Unmarshal(raw, &pbc); err != nil {
		return pbc, fmt.Errorf("protobuf config error: %s", err)
	}
	output := filePathJoinRoot(root, pbc.Output)
	if err := DirExists(output); err != nil {
		return pbc, fmt.Errorf("protobuf output is not exists: %s", pbc.Output)
	}
	return pbc, nil
}
