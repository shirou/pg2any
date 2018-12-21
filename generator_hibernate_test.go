package main

import (
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
