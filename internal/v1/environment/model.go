package environment

type Environment struct {
	Name string `json:"-"`
	GCP  bool   `json:"-"`
}
