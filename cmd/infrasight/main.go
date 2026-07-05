package main

import (
	"github.com/cloudkops/infrasight/internal/app"

	_ "github.com/cloudkops/infrasight/internal/report/json"
	_ "github.com/cloudkops/infrasight/internal/report/markdown"
	_ "github.com/cloudkops/infrasight/internal/report/sarif"
	_ "github.com/cloudkops/infrasight/internal/report/table"

	// register providers by importing them
	_ "github.com/cloudkops/infrasight/providers/docker"
	_ "github.com/cloudkops/infrasight/providers/host"
	_ "github.com/cloudkops/infrasight/providers/kubernetes"
)

func main() {
	app.Execute()
}
