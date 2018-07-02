package new

import (
	"fmt"

	"github.com/spf13/cobra"

	cmdError "github.com/cedriclam/k8snssetup/cmd/k8snssetup/error"
	"github.com/cedriclam/k8snssetup/pkg/common"
)

// NewCmd return new cobra Command for k8snssetup
func NewCmd() *cobra.Command {
	newCmd := &cobra.Command{
		Use:   "new-ns <namespace-name> [required-flags]",
		Short: "Creates a new namespace with new user",
		Long:  ``,
		Run:   newFunc,
	}

	newCmd.Flags().StringArrayVar(&usersCfg, "user", []string{}, "user name")
	newCmd.Flags().StringVar(&kubeConfig, "kubeconfig", "", "kubeconfig file path")
	newCmd.Flags().StringVar(&outputPath, "output", "", "kubeconfig file path")

	return newCmd
}

var usersCfg []string
var kubeConfig string
var outputPath string
var namespace string

func newFunc(cmd *cobra.Command, args []string) {
	if err := parseArgs(args); err != nil {
		cmdError.ExitWithError(cmdError.ExitBadArgs, err)
	}

	if err := common.CreateNamespaceAndUsers(kubeConfig, namespace, usersCfg); err != nil {
		cmdError.ExitWithError(cmdError.ExitError, err)
	}

	fmt.Printf("Namespace '%s' created\n", namespace)
}

func parseArgs(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("new-ns command needs 1 argument")
	}

	namespace = args[0]

	return nil
}
