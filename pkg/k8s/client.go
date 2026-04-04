package k8s

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	CoreV1    corev1client.CoreV1Interface
	Discovery discovery.DiscoveryInterface
	Dynamic   dynamic.Interface
	Config    *rest.Config
	Namespace string
	gvrCache  map[string]*resourceInfo
}

type resourceInfo struct {
	GVR        schema.GroupVersionResource
	Namespaced bool
}

func NewClient(kubeContext, namespace string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		configOverrides.CurrentContext = kubeContext
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	if namespace == "" {
		rawConfig, err := kubeConfig.RawConfig()
		if err != nil {
			return nil, fmt.Errorf("loading raw kubeconfig: %w", err)
		}
		ctx := rawConfig.CurrentContext
		if kubeContext != "" {
			ctx = kubeContext
		}
		if c, ok := rawConfig.Contexts[ctx]; ok && c.Namespace != "" {
			namespace = c.Namespace
		} else {
			namespace = "default"
		}
	}

	coreClient, err := corev1client.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating core/v1 client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating discovery client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &Client{
		CoreV1:    coreClient,
		Discovery: discoveryClient,
		Dynamic:   dynamicClient,
		Config:    config,
		Namespace: namespace,
		gvrCache:  make(map[string]*resourceInfo),
	}, nil
}

// NewClientFromInterfaces creates a Client from pre-built interfaces (for testing).
func NewClientFromInterfaces(coreV1 corev1client.CoreV1Interface, disc discovery.DiscoveryInterface, dyn dynamic.Interface, namespace string) *Client {
	return &Client{
		CoreV1:    coreV1,
		Discovery: disc,
		Dynamic:   dyn,
		Namespace: namespace,
		gvrCache:  make(map[string]*resourceInfo),
	}
}
