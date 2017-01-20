package main

import "testing"

func TestAdd(t *testing.T) {
	if add(1, 2) != 3 {
		t.Error("nooo")
	}
}
func TestSub(t *testing.T) {
	if sub(5, 2) != 2 {
		t.Error("nooo")
	}
}
