package model

type Container struct {
	Name       string
	Image      string
	User       int64
	Privileged bool
	Resources  Resource
	Exposures  []Exposure
}
