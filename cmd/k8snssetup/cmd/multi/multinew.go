package multi

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	cmdError "github.com/cedriclam/k8snssetup/cmd/k8snssetup/error"
	"github.com/cedriclam/k8snssetup/pkg/common"
)

// NewMultiCmd return new cobra Command for k8snssetup
func NewCmd() *cobra.Command {
	newCmd := &cobra.Command{
		Use:   "multi <number of namespace> [required-flags]",
		Short: "Creates a new namespace with new user",
		Long:  ``,
		Run:   newFunc,
	}

	newCmd.Flags().StringVar(&userPrefix, "user-prefix", "user", "user prefix")
	newCmd.Flags().StringVar(&nsPrefix, "ns-prefix", "project", "namespace prefix")
	newCmd.Flags().StringVar(&kubeConfig, "kubeconfig", "", "kubeconfig file path")
	newCmd.Flags().StringVar(&outputPath, "output", "", "kubeconfig file path")

	return newCmd
}

var (
	nbProject  int64
	userPrefix string
	kubeConfig string
	outputPath string
	nsPrefix   string
)

func newFunc(cmd *cobra.Command, args []string) {
	if err := parseArgs(args); err != nil {
		cmdError.ExitWithError(cmdError.ExitBadArgs, err)
	}

	if err := process(nbProject, nsPrefix, userPrefix); err != nil {
		cmdError.ExitWithError(cmdError.ExitError, err)
	}

}

func process(nbProject int64, nsPrefix, userPrefix string) error {
	var i int64
	for i = 1; i <= nbProject; i++ {
		userName := fmt.Sprintf("%s%d", userPrefix, i)
		nsName := fmt.Sprintf("%s%d", nsPrefix, i)
		if err := common.CreateNamespaceAndUsers(kubeConfig, nsName, []string{userName}); err != nil {
			return fmt.Errorf("unable to create ns:%s user:%s, err:%s", nsName, userName, err)
		}

		fmt.Printf("- ns:%s and user:%s created\n", nsName, userName)
	}
	fmt.Println("Done")
	return nil
}

func parseArgs(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("multi command needs 1 argument, the number of namespace to create")
	}

	i, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil {
		return err
	}
	nbProject = i

	return nil
}
