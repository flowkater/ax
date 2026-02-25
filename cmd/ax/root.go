package ax

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "ax",
		Short: "ax v2 MVP CLI",
		Long:  "ax v2 MVP command-line interface for propose-plan-run-verify-archive workflows.",
	}

	root.AddCommand(newProposeCmd())
	root.AddCommand(newPlanCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newVerifyCmd())
	root.AddCommand(newArchiveCmd())
	root.AddCommand(newDiscoverCmd())
	root.AddCommand(newQuickCmd())
	root.AddCommand(newStateCmd())

	return root
}
