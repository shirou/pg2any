package main

import (
	"testing"
)

func TestSnakeToUpperCamel(t *testing.T) {
	if SnakeToUpperCamel("foo_bar_coo") != "FooBarCoo" {
		t.Errorf("not match: %s", SnakeToUpperCamel("foo_bar_coo"))
	}
}

func TestPartContains(t *testing.T) {
	s := []string{"aaa", "bbb", "abcde", "b$"}
	if partContainsRegex(s, "a") != false {
		t.Error("should be false")
	}
	if partContainsRegex(s, "aaaa") != true {
		t.Error("should be true")
	}
	if partContainsRegex(s, "cd") != false {
		t.Error("should be false")
	}
	if partContainsRegex(s, "bcd") != false {
		t.Error("should be false")
	}
	if partContainsRegex(s, "abc") != false {
		t.Error("should be false")
	}
	if partContainsRegex(s, "abb") != true {
		t.Error("should be true")
	}
}
