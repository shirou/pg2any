package main

import "testing"

func TestSnakeToUpperCamel(t *testing.T) {
	if SnakeToUpperCamel("foo_bar_coo") != "FooBarCoo" {
		t.Errorf("not match", SnakeToUpperCamel("foo_bar_coo"))
	}
}
