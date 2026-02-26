package ax

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "ax",
		Short:        "ax v2 MVP CLI",
		Long:         "ax v2 MVP command-line interface for propose-plan-run-verify-archive workflows.",
		SilenceUsage: true,
	}
	root.PersistentFlags().String("runtime-mode", "", "Runtime mode override (single|shared|worktree|auto); defaults to AX_RUNTIME_MODE or single")
	root.PersistentFlags().String("session-id", "", "Explicit runtime session id (optional)")
	root.PersistentFlags().String("cluster-id", "", "Runtime cluster id override (optional; defaults to AX_CLUSTER_ID/AX_RUNTIME_CLUSTER/local)")
	root.PersistentFlags().String("node-id", "", "Runtime node id override (optional; defaults to AX_NODE_ID or hostname:pid)")

	root.AddCommand(newProposeCmd())
	root.AddCommand(newPlanCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newVerifyCmd())
	root.AddCommand(newArchiveCmd())
	root.AddCommand(newDiscoverCmd())
	root.AddCommand(newQuickCmd())
	root.AddCommand(newStateCmd())
	root.AddCommand(newRecoverCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newReviewCmd())
	root.AddCommand(newCompoundCmd())

	return root
}
