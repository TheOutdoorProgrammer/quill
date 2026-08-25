package main

import (
	"reflect"
	"testing"
)

func TestParseBundleIdentifiers(t *testing.T) {
	want := []string{"com.example.child", "com.example.parent"}
	got, err := parseBundleIdentifiers("com.example.parent\ncom.example.child, com.example.parent")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseBundleIdentifiersRejectsWildcard(t *testing.T) {
	if _, err := parseBundleIdentifiers("com.example.*"); err == nil {
		t.Fatal("expected wildcard bundle identifier to be rejected")
	}
}
