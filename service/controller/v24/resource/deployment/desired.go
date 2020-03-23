package deployment

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	v1 "k8s.io/api/apps/v1"

	"github.com/giantswarm/kvm-operator/service/controller/v24/key"
)

func (r *Resource) GetDesiredState(ctx context.Context, obj interface{}) (interface{}, error) {
	customResource, err := key.ToCustomObject(obj)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", "computing the new deployments")

	var deployments []*v1.Deployment

	{
		masterDeployments, err := newMasterDeployments(customResource, r.dnsServers, r.ntpServers)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		deployments = append(deployments, masterDeployments...)

		workerDeployments, err := newWorkerDeployments(customResource, r.dnsServers, r.ntpServers)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		deployments = append(deployments, workerDeployments...)
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("computed the %d new deployments", len(deployments)))

	return deployments, nil
}