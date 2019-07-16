package main

import (
	"database/sql"
	"testing"
)

func TestDecapitalize(t *testing.T) {
	ff := [][]string{
		[]string{"FooBar", "fooBar"},
		[]string{"X", "X"},
		[]string{"URL", "URL"},
		[]string{"o_foo_bar", "OFooBar"},
	}
	for _, d := range ff {
		if actual := decapitalize(SnakeToUpperCamel(d[0])); actual != d[1] {
			t.Errorf("expected: %s, actual: %s", d[0], actual)
		}
	}
}

func TestIsSequence(t *testing.T) {
	col := Column{
		PrimaryKey: true,
		DefaultValue: sql.NullString{
			String: "nextval('foo_bar_id_seq'::regclass)",
			Valid:  true,
		},
	}
	if isSequence(col) == false {
		t.Errorf("unexpected")
	}
}
