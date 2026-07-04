package config

import (
	"reflect"
	"testing"
)

func TestApplyDefaultsAddsGopherSkillsWhenOmitted(t *testing.T) {
	var cfg Config
	cfg.applyDefaults()

	want := []string{"skills/gopher"}
	if !reflect.DeepEqual(cfg.Tools.GopherSkills, want) {
		t.Fatalf("GopherSkills = %v, want %v", cfg.Tools.GopherSkills, want)
	}
}

func TestApplyDefaultsPreservesExplicitEmptyGopherSkills(t *testing.T) {
	cfg := Config{}
	cfg.Tools.GopherSkills = []string{}
	cfg.applyDefaults()

	if cfg.Tools.GopherSkills == nil || len(cfg.Tools.GopherSkills) != 0 {
		t.Fatalf("GopherSkills = %#v, want explicit empty slice", cfg.Tools.GopherSkills)
	}
}
