package clusterrolebinding

import (
	"context"

	"github.com/giantswarm/microerror"
	apiv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Resource) ApplyCreateChange(ctx context.Context, obj, createChange interface{}) error {
	clusterRoleBindingsToCreate, err := toClusterRoleBindings(createChange)
	if err != nil {
		return microerror.Mask(err)
	}

	// Create the cluster role bindings in the Kubernetes API.
	if len(clusterRoleBindingsToCreate) != 0 {
		r.logger.Debugf(ctx, "creating the cluster role bindings in the Kubernetes API")

		for _, clusterRoleBinding := range clusterRoleBindingsToCreate {
			_, err := r.k8sClient.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, v1.CreateOptions{})
			if apierrors.IsAlreadyExists(err) {
			} else if err != nil {
				return microerror.Mask(err)
			}
		}

		r.logger.Debugf(ctx, "created the cluster role bindings in the Kubernetes API")
	} else {
		r.logger.Debugf(ctx, "the cluster role bindings do not need to be created in the Kubernetes API")
	}

	return nil
}

func (r *Resource) newCreateChange(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, error) {
	currentClusterRoleBindings, err := toClusterRoleBindings(currentState)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	desiredClusterRoleBindings, err := toClusterRoleBindings(desiredState)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	r.logger.Debugf(ctx, "finding out which cluster role bindings have to be created")

	var clusterRoleBindingsToCreate []*apiv1.ClusterRoleBinding

	for _, desiredClusterRoleBinding := range desiredClusterRoleBindings {
		if !containsClusterRoleBinding(currentClusterRoleBindings, desiredClusterRoleBinding) {
			clusterRoleBindingsToCreate = append(clusterRoleBindingsToCreate, desiredClusterRoleBinding)
		}
	}

	r.logger.Debugf(ctx, "found %d cluster role bindings that have to be created", len(clusterRoleBindingsToCreate))

	return clusterRoleBindingsToCreate, nil
}
