// Package config holds the settings, set in code the way an ini file is read at start.
package config

// Settings of the program.
type Settings struct {
	Table  string
	Shell  string
	Home   string
	Limits map[string]string
}

// Default is what the program starts with.
var Default = &Settings{Table: "users", Shell: "/bin/sh", Home: "/", Limits: map[string]string{"rows": "100"}}
