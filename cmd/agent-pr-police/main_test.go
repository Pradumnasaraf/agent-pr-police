package main

import (
	"reflect"
	"testing"
)

func TestSplitList(t *testing.T) {
	tests := map[string][]string{
		"":                  nil,
		"@alice":            {"alice"},
		"@alice @bob":       {"alice", "bob"},
		"alice, bob,carol":  {"alice", "bob", "carol"},
		"@org/team\n@alice": {"org/team", "alice"},
		"  @a ,  , @b  ":    {"a", "b"},
	}
	for in, want := range tests {
		if got := splitList(in); !reflect.DeepEqual(got, want) {
			t.Errorf("splitList(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestDropAuthor(t *testing.T) {
	got := dropAuthor([]string{"Alice", "bob", "org/team"}, "alice")
	want := []string{"bob", "org/team"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dropAuthor = %v, want %v", got, want)
	}
}

func TestParseReviewers(t *testing.T) {
	users, teams := parseReviewers([]string{"alice", "org/security", "bob", "org/"})
	if !reflect.DeepEqual(users, []string{"alice", "bob"}) {
		t.Errorf("users = %v", users)
	}
	if !reflect.DeepEqual(teams, []string{"security"}) {
		t.Errorf("teams = %v", teams)
	}
}
