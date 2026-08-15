// Package optimum owns discovery and validation of user-installed Optimum clients.
package optimum

type Installation struct {
	Path             string
	Executable       string
	WorkingDirectory string
	GameVersion      string
	Exclusive        bool
}

type Status struct {
	Path        string
	Executable  string
	GameVersion string
	Ready       bool
	Message     string
}
