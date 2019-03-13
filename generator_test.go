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
	s := []string{"aaa", "bbb", "abcde"}
	if partContains(s, "a") != false {
		t.Error("should be false")
	}
	if partContains(s, "aaaa") != true {
		t.Error("should be true")
	}
	if partContains(s, "cd") != false {
		t.Error("should be false")
	}
	if partContains(s, "bcd") != false {
		t.Error("should be false")
	}
}
