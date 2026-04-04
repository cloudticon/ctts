package k8s

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Clientset kubernetes.Interface
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

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes clientset: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &Client{
		Clientset: clientset,
		Dynamic:   dynamicClient,
		Config:    config,
		Namespace: namespace,
		gvrCache:  make(map[string]*resourceInfo),
	}, nil
}

// NewClientFromInterfaces creates a Client from pre-built interfaces (for testing).
func NewClientFromInterfaces(clientset kubernetes.Interface, dyn dynamic.Interface, namespace string) *Client {
	return &Client{
		Clientset: clientset,
		Dynamic:   dyn,
		Namespace: namespace,
		gvrCache:  make(map[string]*resourceInfo),
	}
}
