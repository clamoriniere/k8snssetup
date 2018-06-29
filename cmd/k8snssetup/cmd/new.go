package cmd

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/spf13/cobra"

	kcore_v1 "k8s.io/api/core/v1"
	krbac_v1 "k8s.io/api/rbac/v1"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	config_v1 "k8s.io/client-go/tools/clientcmd/api"

	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cmdError "github.com/cedriclam/k8snssetup/cmd/k8snssetup/error"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// NewNewCmd return new cobra Command for k8snssetup
func NewNewCmd() *cobra.Command {
	newCmd := &cobra.Command{
		Use:   "new-ns <namespace-name> [required-flags]",
		Short: "Creates a new namespace with new user",
		Long:  ``,
		Run:   newFunc,
	}

	newCmd.Flags().StringArrayVar(&users, "user", []string{}, "user name")
	newCmd.Flags().StringVar(&kubeConfig, "kubeconfig", "", "kubeconfig file path")
	newCmd.Flags().StringVar(&outputPath, "output", "", "kubeconfig file path")

	return newCmd
}

var users []string
var kubeConfig string
var outputPath string
var namespace string

func newFunc(cmd *cobra.Command, args []string) {
	if err := parseArgs(args); err != nil {
		cmdError.ExitWithError(cmdError.ExitBadArgs, err)
	}

	if err := process(namespace); err != nil {
		cmdError.ExitWithError(cmdError.ExitError, err)
	}

	fmt.Printf("Namespace '%s' created\n", namespace)
}

func process(namespace string) error {
	kubeConfig, err := initKubeConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("unable to initialize kubeConfig: %v", err)
	}
	kubeClient, err := clientset.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("unable to initialize kubeConfig: %v", err)
	}

	// Create Role for service/proxy (dashboard in kube-system)
	proxyRoleName := "proxyUser"
	proxyNamespace := "kube-system"
	if err = retry(3, time.Second, func() error {
		return createRole(kubeClient, proxyNamespace, proxyRoleName, []string{krbac_v1.VerbAll}, []string{krbac_v1.APIGroupAll}, []string{"services/proxy"})
	}); err != nil {
		return err
	}

	// start create namespace if needed
	if err = createNamespace(kubeClient, namespace); err != nil {
		return fmt.Errorf("unable create the namesapce %s: %v", namespace, err)
	}

	for _, user := range users {
		saName := fmt.Sprintf("%s", user)
		sa, err := createServiceAccount(kubeClient, namespace, saName)
		if err != nil {
			return fmt.Errorf("unable create the service account %s: %v", saName, err)
		}

		var secretName string
		for _, secret := range sa.Secrets {
			secretName = secret.Name
		}
		if len(secretName) == 0 {
			return fmt.Errorf("unable to get secret name from service account %s: %v", saName, err)
		}
		var secret *kcore_v1.Secret
		var token []byte
		var ok bool
		getTokenFunc := func() error {
			secret, err = kubeClient.CoreV1().Secrets(namespace).Get(secretName, meta_v1.GetOptions{})
			if err != nil {
				return err
			}
			token, ok = secret.Data["token"]
			if !ok {
				return fmt.Errorf("unable to get token from secret %s", secretName)
			}

			return nil
		}
		if err = retry(3, 2*time.Second, getTokenFunc); err != nil {
			return err
		}

		// create role and rolebinding
		if err = retry(3, time.Second, func() error {
			return createRole(kubeClient, namespace, user, []string{krbac_v1.VerbAll}, []string{krbac_v1.APIGroupAll}, []string{krbac_v1.ResourceAll})
		}); err != nil {
			return err
		}
		if err = retry(3, time.Second, func() error { return createRoleBinding(kubeClient, namespace, user, user, namespace, user) }); err != nil {
			return err
		}
		// Create RoleBinding for service/proxy
		if err = retry(3, time.Second, func() error {
			return createRoleBinding(kubeClient, proxyNamespace, fmt.Sprintf("%s-%s-%s", "svc-proxy", namespace, user), proxyRoleName, namespace, user)
		}); err != nil {
			return err
		}

		clusterName := "default"
		cluster := config_v1.NewCluster()
		cluster.Server = kubeConfig.Host
		if len(kubeConfig.CertData) > 0 {
			cluster.CertificateAuthorityData = kubeConfig.CertData
		}
		if len(kubeConfig.CAFile) > 0 {
			cluster.CertificateAuthority = kubeConfig.CAFile
		}
		if len(kubeConfig.CAData) > 0 {
			cluster.CertificateAuthorityData = kubeConfig.CAData
		}

		context := config_v1.NewContext()
		context.Cluster = clusterName
		context.AuthInfo = user
		context.Namespace = namespace

		// create new ConfigFile
		cfg := config_v1.Config{
			Kind:           "Config",
			APIVersion:     "v1",
			AuthInfos:      map[string]*config_v1.AuthInfo{user: {Token: string(token)}},
			Clusters:       map[string]*config_v1.Cluster{clusterName: cluster},
			Contexts:       map[string]*config_v1.Context{clusterName: context},
			CurrentContext: clusterName,
		}

		filePath := fmt.Sprintf("%s-%s.kubeconfig.yaml", namespace, user)

		if err = clientcmd.WriteToFile(cfg, filePath); err != nil {
			return fmt.Errorf("unable to write the kubeconfig: %s, err: %v", filePath, err)
		}
	}

	return nil
}

func createNamespace(kubeClient clientset.Interface, ns string) error {
	_, err := kubeClient.CoreV1().Namespaces().Get(ns, meta_v1.GetOptions{})
	if errors.IsNotFound(err) {
		newNs := kcore_v1.Namespace{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: ns,
			},
		}
		_, err = kubeClient.CoreV1().Namespaces().Create(&newNs)
	}

	if err != nil {
		return err
	}

	return nil
}

func createServiceAccount(kubeClient clientset.Interface, ns string, saName string) (*kcore_v1.ServiceAccount, error) {
	sa, err := kubeClient.CoreV1().ServiceAccounts(ns).Get(saName, meta_v1.GetOptions{})
	if errors.IsNotFound(err) {
		newSA := kcore_v1.ServiceAccount{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: saName,
			},
		}
		sa, err = kubeClient.CoreV1().ServiceAccounts(ns).Create(&newSA)
	}

	if err != nil {
		return nil, err
	}

	var secretName string
	for _, secret := range sa.Secrets {
		secretName = secret.Name
	}
	if len(secretName) == 0 {
		return nil, fmt.Errorf("unable to get secret name from service account %s: %v", saName, err)
	}

	return sa, nil
}

func createRole(kubeClient clientset.Interface, ns, user string, verbs, apigroups, resources []string) error {
	_, err := kubeClient.RbacV1().Roles(ns).Get(user, meta_v1.GetOptions{})
	if errors.IsNotFound(err) {
		role := krbac_v1.Role{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: user,
			},
			Rules: []krbac_v1.PolicyRule{
				{
					Verbs:     verbs,
					APIGroups: apigroups,
					Resources: resources,
				},
			},
		}
		_, err = kubeClient.RbacV1().Roles(ns).Create(&role)
	}

	if err != nil {
		return fmt.Errorf("unable to create Role, err: %v", err)
	}

	return nil
}

func createRoleBinding(kubeClient clientset.Interface, ns, roleBindingName, roleName, userNs, user string) error {
	_, err := kubeClient.RbacV1().RoleBindings(ns).Get(user, meta_v1.GetOptions{})
	if errors.IsNotFound(err) {
		role := krbac_v1.RoleBinding{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      roleBindingName,
				Namespace: ns,
			},
			RoleRef: krbac_v1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     roleName,
			},
			Subjects: []krbac_v1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      user,
					Namespace: userNs,
				},
			},
		}
		_, err = kubeClient.RbacV1().RoleBindings(ns).Create(&role)
	}

	if err != nil {
		return fmt.Errorf("unable to create RoleBindings, err: %v", err)
	}

	return nil
}

func parseArgs(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("new-ns command needs 1 argument")
	}

	namespace = args[0]

	return nil
}

func initKubeConfig(cfgFile string) (*rest.Config, error) {
	if len(cfgFile) > 0 {
		return clientcmd.BuildConfigFromFlags("", cfgFile) // out of cluster config
	}
	return rest.InClusterConfig()
}

func retry(attempts int, sleep time.Duration, f func() error) error {
	if err := f(); err != nil {
		if s, ok := err.(stop); ok {
			// Return the original error for later checking
			return s.error
		}

		if attempts--; attempts > 0 {
			// Add some randomness to prevent creating a Thundering Herd
			jitter := time.Duration(rand.Int63n(int64(sleep)))
			sleep = sleep + jitter/2

			time.Sleep(sleep)
			return retry(attempts, 2*sleep, f)
		}
		return err
	}

	return nil
}

type stop struct {
	error
}
