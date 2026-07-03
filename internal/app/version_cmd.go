package app

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version and commit are overridden at build time via -ldflags
// "-X github.com/cloudkops/infrasight/internal/app.version=..." (see Makefile) —
// they must stay vars, not consts, for -X to have anything to target. commit
// defaults to "none" for `go run`/`go build` invocations that skip the Makefile.
var (
	version = "0.1.0"
	commit  = "none"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the infrasight version",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(fmt.Sprintf("%s (commit %s)", version, commit))
		},
	}
}
