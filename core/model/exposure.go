package model

type Exposure struct {
	Type string // port, hostNetwork, publicIP
	Port int32
}
