package deployment

import (
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/tenantcluster/v4/pkg/tenantcluster"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Name = "deployment"
)

type Config struct {
	CtrlClient    client.Client
	Logger        micrologger.Logger
	TenantCluster tenantcluster.Interface
}

type Resource struct {
	ctrlClient    client.Client
	logger        micrologger.Logger
	tenantCluster tenantcluster.Interface
}

func New(config Config) (*Resource, error) {
	if config.CtrlClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.CtrlClient must not be empty", config)
	}
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Logger must not be empty", config)
	}

	r := &Resource{
		ctrlClient: config.CtrlClient,
		logger:     config.Logger,
	}

	return r, nil
}

func (r *Resource) Name() string {
	return Name
}
