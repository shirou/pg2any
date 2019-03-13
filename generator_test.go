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
	if partContains(s, "a") != true {
		t.Error("should be true")
	}
	if partContains(s, "hoge") != false {
		t.Error("should be false")
	}
	if partContains(s, "cd") != true {
		t.Error("should be true")
	}
	if partContains(s, "bcd") != true {
		t.Error("should be true")
	}
}
