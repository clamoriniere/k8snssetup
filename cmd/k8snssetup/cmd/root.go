package cmd

import (
	"github.com/spf13/cobra"

	"github.com/cedriclam/k8snssetup/version"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "k8snssetup",
		Short:   "A tools generate kubeconfig file for a specific namespace and users",
		Version: version.Version,
	}

	cmd.AddCommand(NewNewCmd())

	return cmd
}
