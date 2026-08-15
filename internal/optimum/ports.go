package optimum

type Locator interface {
	Detect() (Installation, error)
	Inspect(string) (Installation, error)
	GameVersion(string) string
	InUse(Installation) (bool, error)
}
