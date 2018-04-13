package v11

import (
	"context"

	"github.com/cenkalti/backoff"
	"github.com/giantswarm/certs"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/operatorkit/framework"
	"github.com/giantswarm/operatorkit/framework/context/updateallowedcontext"
	"github.com/giantswarm/operatorkit/framework/resource/metricsresource"
	"github.com/giantswarm/operatorkit/framework/resource/retryresource"
	"github.com/giantswarm/randomkeys"
	"k8s.io/client-go/kubernetes"

	"github.com/giantswarm/kvm-operator/service/controller/v11/cloudconfig"
	"github.com/giantswarm/kvm-operator/service/controller/v11/key"
	"github.com/giantswarm/kvm-operator/service/controller/v11/resource/clusterrolebinding"
	"github.com/giantswarm/kvm-operator/service/controller/v11/resource/configmap"
	"github.com/giantswarm/kvm-operator/service/controller/v11/resource/deployment"
	"github.com/giantswarm/kvm-operator/service/controller/v11/resource/ingress"
	"github.com/giantswarm/kvm-operator/service/controller/v11/resource/namespace"
	"github.com/giantswarm/kvm-operator/service/controller/v11/resource/pvc"
	"github.com/giantswarm/kvm-operator/service/controller/v11/resource/service"
	"github.com/giantswarm/kvm-operator/service/controller/v11/resource/serviceaccount"
)

type ClusterResourceSetConfig struct {
	CertsSearcher      certs.Interface
	K8sClient          kubernetes.Interface
	Logger             micrologger.Logger
	RandomkeysSearcher randomkeys.Interface

	GuestUpdateEnabled bool
	ProjectName        string
}

func NewClusterResourceSet(config ClusterResourceSetConfig) (*framework.ResourceSet, error) {
	var err error

	var cloudConfig *cloudconfig.CloudConfig
	{
		c := cloudconfig.DefaultConfig()

		c.Logger = config.Logger

		cloudConfig, err = cloudconfig.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var clusterRoleBindingResource framework.Resource
	{
		c := clusterrolebinding.Config{
			K8sClient: config.K8sClient,
			Logger:    config.Logger,
		}

		ops, err := clusterrolebinding.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		clusterRoleBindingResource, err = toCRUDResource(config.Logger, ops)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var namespaceResource framework.Resource
	{
		c := namespace.DefaultConfig()

		c.K8sClient = config.K8sClient
		c.Logger = config.Logger

		ops, err := namespace.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		namespaceResource, err = toCRUDResource(config.Logger, ops)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var serviceAccountResource framework.Resource
	{
		c := serviceaccount.DefaultConfig()

		c.K8sClient = config.K8sClient
		c.Logger = config.Logger

		ops, err := serviceaccount.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		serviceAccountResource, err = toCRUDResource(config.Logger, ops)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var configMapResource framework.Resource
	{
		c := configmap.DefaultConfig()

		c.CertSearcher = config.CertsSearcher
		c.CloudConfig = cloudConfig
		c.K8sClient = config.K8sClient
		c.KeyWatcher = config.RandomkeysSearcher
		c.Logger = config.Logger

		ops, err := configmap.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		configMapResource, err = toCRUDResource(config.Logger, ops)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var deploymentResource framework.Resource
	{
		c := deployment.DefaultConfig()

		c.K8sClient = config.K8sClient
		c.Logger = config.Logger

		ops, err := deployment.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		deploymentResource, err = toCRUDResource(config.Logger, ops)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var ingressResource framework.Resource
	{
		c := ingress.DefaultConfig()

		c.K8sClient = config.K8sClient
		c.Logger = config.Logger

		ops, err := ingress.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		ingressResource, err = toCRUDResource(config.Logger, ops)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var pvcResource framework.Resource
	{
		c := pvc.DefaultConfig()

		c.K8sClient = config.K8sClient
		c.Logger = config.Logger

		ops, err := pvc.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		pvcResource, err = toCRUDResource(config.Logger, ops)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var serviceResource framework.Resource
	{
		c := service.DefaultConfig()

		c.K8sClient = config.K8sClient
		c.Logger = config.Logger

		ops, err := service.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		serviceResource, err = toCRUDResource(config.Logger, ops)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	resources := []framework.Resource{
		clusterRoleBindingResource,
		namespaceResource,
		serviceAccountResource,
		configMapResource,
		deploymentResource,
		ingressResource,
		pvcResource,
		serviceResource,
	}

	{
		c := retryresource.WrapConfig{
			BackOffFactory: func() backoff.BackOff { return backoff.WithMaxTries(backoff.NewExponentialBackOff(), uint64(3)) },
			Logger:         config.Logger,
		}

		resources, err = retryresource.Wrap(resources, c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	{
		c := metricsresource.WrapConfig{
			Name: config.ProjectName,
		}

		resources, err = metricsresource.Wrap(resources, c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	handlesFunc := func(obj interface{}) bool {
		kvmConfig, err := key.ToCustomObject(obj)
		if err != nil {
			return false
		}

		if key.VersionBundleVersion(kvmConfig) == VersionBundle().Version {
			return true
		}

		return false
	}

	initCtxFunc := func(ctx context.Context, obj interface{}) (context.Context, error) {
		if config.GuestUpdateEnabled {
			updateallowedcontext.SetUpdateAllowed(ctx)
		}

		return ctx, nil
	}

	var clusterResourceSet *framework.ResourceSet
	{
		c := framework.ResourceSetConfig{
			Handles:   handlesFunc,
			InitCtx:   initCtxFunc,
			Logger:    config.Logger,
			Resources: resources,
		}

		clusterResourceSet, err = framework.NewResourceSet(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	return clusterResourceSet, nil
}

func toCRUDResource(logger micrologger.Logger, ops framework.CRUDResourceOps) (*framework.CRUDResource, error) {
	c := framework.CRUDResourceConfig{
		Logger: logger,
		Ops:    ops,
	}

	r, err := framework.NewCRUDResource(c)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return r, nil
}