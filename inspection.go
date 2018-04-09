package main

import (
	"database/sql"
	"fmt"

	"github.com/pkg/errors"
)

type InspectResult struct {
	Tables []Table
	Types  []Type
}

type Table struct {
	Schema      string
	Name        string
	Comment     sql.NullString
	DataType    string
	AutoGenPk   bool
	PrimaryKeys []Column
	Columns     []Column
}

type Column struct {
	FieldOrdinal  int            // field ordinal
	Name          string         // column name
	Comment       sql.NullString // comment
	DataType      string         // data type
	NotNull       bool           // not null
	DefaultValue  sql.NullString // default value
	PrimaryKey    bool
	Unique        bool
	Constraint    sql.NullString
	ConstraintSrc sql.NullString
	ForignTable   sql.NullString
}

type Type struct {
	DataType string
	Name     string
	Comment  sql.NullString
	NotNull  bool
	Values   []string
}

func (ins InspectResult) FindType(name string) (Type, error) {
	for _, typ := range ins.Types {
		if typ.Name == name {
			return typ, nil
		}
	}
	return Type{}, fmt.Errorf("not found")
}

func Inspect(db *sql.DB) (InspectResult, error) {
	var ret InspectResult

	tables, err := getTables(db, "public")
	if err != nil {
		return ret, errors.Wrap(err, "Inspect")
	}
	ret.Tables = tables

	types, err := getTypes(db)
	if err != nil {
		return ret, errors.Wrap(err, "Inspect")
	}
	ret.Types = types

	return ret, nil
}

func getTables(db *sql.DB, schema string) ([]Table, error) {
	// https://github.com/achiku/dgw/blob/master/dgw.go
	q := `SELECT
c.relkind AS type,
c.relname AS table_name,
obj_description(c.oid)
FROM pg_class c
JOIN ONLY pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1
AND c.relkind = 'r'
ORDER BY c.relname
`
	rows, err := db.Query(q, schema)
	if err != nil {
		return nil, err
	}
	var tbs []Table
	for rows.Next() {
		t := Table{
			Schema: schema,
		}
		if err := rows.Scan(&t.DataType, &t.Name, &t.Comment); err != nil {
			return nil, errors.Wrap(err, "failed to scan of "+t.Name)
		}
		cols, err := getColumns(db, schema, t.Name, false)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get columns of %s", t.Name))
		}
		t.Columns = cols
		tbs = append(tbs, t)
	}
	return tbs, nil
}

func getColumns(db *sql.DB, schema, table string, sys bool) ([]Column, error) {
	// https://github.com/xo/xo/blob/master/models/column.xo.go#L21
	const sqlstr = `SELECT
a.attnum,
a.attname,
col_description(c.oid, a.attnum),
format_type(a.atttypid, a.atttypmod),
a.attnotnull,
COALESCE(pg_get_expr(ad.adbin, ad.adrelid), ''),
ct.contype,
pg_catalog.pg_get_constraintdef(ct.oid, true),
cc.relname
FROM pg_attribute a
JOIN ONLY pg_class c ON c.oid = a.attrelid
JOIN ONLY pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_constraint ct ON ct.conrelid = c.oid AND a.attnum = ANY(ct.conkey)
LEFT JOIN pg_attrdef ad ON ad.adrelid = c.oid AND ad.adnum = a.attnum
LEFT JOIN pg_class cc ON cc.oid = ct.conrelid
WHERE a.attisdropped = false AND n.nspname = $1 AND c.relname = $2 AND ($3 OR a.attnum > 0)
ORDER BY a.attnum`
	q, err := db.Query(sqlstr, schema, table, sys)
	if err != nil {
		return nil, errors.Wrap(err, "columns query")
	}
	tmp := make(map[string]Column)
	var order []string
	for q.Next() {
		c := Column{}

		// scan
		err = q.Scan(
			&c.FieldOrdinal,
			&c.Name,
			&c.Comment,
			&c.DataType,
			&c.NotNull,
			&c.DefaultValue,
			&c.Constraint,
			&c.ConstraintSrc,
			&c.ForignTable,
		)
		if err != nil {
			return nil, errors.Wrap(err, "columns scan")
		}
		switch c.Constraint.String {
		case "p":
			c.PrimaryKey = true
		case "u":
			c.Unique = true
		}
		o, exists := tmp[c.Name]
		if !exists {
			o = c
		} else {
			o.PrimaryKey = o.PrimaryKey || c.PrimaryKey
			o.Unique = o.Unique || c.Unique
			o.ForignTable = c.ForignTable
			o.ConstraintSrc = c.ConstraintSrc
		}

		tmp[c.Name] = o
		if !contains(order, c.Name) {
			order = append(order, c.Name)
		}
	}

	var ret []Column
	for _, o := range order {
		ret = append(ret, tmp[o])
	}

	return ret, nil
}

func getTypes(db *sql.DB) ([]Type, error) {
	q := `
SELECT
t.typname as type,
obj_description(t.oid),
t.typnotnull
FROM        pg_type t
LEFT JOIN   pg_catalog.pg_namespace n ON n.oid = t.typnamespace
WHERE       (t.typrelid = 0 OR (SELECT c.relkind = 'c' FROM pg_catalog.pg_class c WHERE c.oid = t.typrelid))
AND     NOT EXISTS(SELECT 1 FROM pg_catalog.pg_type el WHERE el.oid = t.typelem AND el.typarray = t.oid)
AND     n.nspname NOT IN ('pg_catalog', 'information_schema')
`

	rows, err := db.Query(q)
	if err != nil {
		return nil, errors.Wrap(err, "type query")
	}
	var typs []Type
	for rows.Next() {
		var t Type
		if err := rows.Scan(&t.Name, &t.Comment, &t.NotNull); err != nil {
			return nil, errors.Wrap(err, "type scan")
		}

		values, err := getEnum(db, t.Name)
		if err != nil {
			return nil, errors.Wrap(err, "get Enum")
		}
		t.Values = values
		typs = append(typs, t)
	}
	return typs, nil
}

func getEnum(db *sql.DB, typName string) ([]string, error) {
	q := `
SELECT pg_enum.enumlabel AS enumlabel
FROM pg_type
JOIN pg_enum
     ON pg_enum.enumtypid = pg_type.oid
WHERE
     pg_type.typname = $1
ORDER BY pg_enum.enumsortorder
`
	rows, err := db.Query(q, typName)
	if err != nil {
		return nil, errors.Wrap(err, "enum query")
	}
	var values []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, errors.Wrap(err, "enum scan")
		}
		values = append(values, t)
	}
	return values, nil

}
