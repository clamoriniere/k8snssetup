package common

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	kcore_v1 "k8s.io/api/core/v1"
	krbac_v1 "k8s.io/api/rbac/v1"

	clientset "k8s.io/client-go/kubernetes"
	// for supporting client auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	config_v1 "k8s.io/client-go/tools/clientcmd/api"

	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// CreateNamespaceAndUsers used to create Namespace and User resources
func CreateNamespaceAndUsers(kubeConfigfile string, namespace string, users []string) error {
	kubeConfig, err := initKubeConfig(kubeConfigfile)
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
		var sa *kcore_v1.ServiceAccount
		var err error
		if err = retry(3, time.Second, func() error {
			sa, err = createServiceAccount(kubeClient, namespace, saName)
			return err
		}); err != nil {
			return err
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

func initKubeConfig(cfgFile string) (*rest.Config, error) {
	kubeconfigFilePath := getKubeConfigDefaultPath(getHomePath(), cfgFile)
	if len(kubeconfigFilePath) == 0 {
		return nil, fmt.Errorf("error initializing config. The KUBECONFIG environment variable must be defined")
	}

	config, err := configFromPath(kubeconfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("error obtaining kubectl config: %v", err)
	}

	return config.ClientConfig()
}

func configFromPath(path string) (clientcmd.ClientConfig, error) {
	rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: path}
	credentials, err := rules.Load()
	if err != nil {
		return nil, fmt.Errorf("the provided credentials %q could not be loaded: %v", path, err)
	}

	overrides := &clientcmd.ConfigOverrides{
		Context: clientcmdapi.Context{
			Namespace: os.Getenv("KUBECTL_PLUGINS_GLOBAL_FLAG_NAMESPACE"),
		},
	}

	context := os.Getenv("KUBECTL_PLUGINS_GLOBAL_FLAG_CONTEXT")
	if len(context) > 0 {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		return clientcmd.NewNonInteractiveClientConfig(*credentials, context, overrides, rules), nil
	}
	return clientcmd.NewDefaultClientConfig(*credentials, overrides), nil
}

func getHomePath() string {
	home := os.Getenv("HOME")
	if runtime.GOOS == "windows" {
		home = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
	}

	return home
}

func getKubeConfigDefaultPath(home string, kubeconfigPath string) string {
	kubeconfig := filepath.Join(home, ".kube", "config")

	kubeconfigEnv := os.Getenv("KUBECONFIG")
	if len(kubeconfigEnv) > 0 {
		kubeconfig = kubeconfigEnv
	}

	if len(kubeconfigPath) > 0 {
		kubeconfig = kubeconfigPath
	}
	return kubeconfig
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
