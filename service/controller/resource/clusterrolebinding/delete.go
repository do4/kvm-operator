package clusterrolebinding

import (
	"context"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/operatorkit/v4/pkg/resource/crud"
	apiv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Resource) ApplyDeleteChange(ctx context.Context, obj, deleteChange interface{}) error {
	clusterRoleBindingsToDelete, err := toClusterRoleBindings(deleteChange)
	if err != nil {
		return microerror.Mask(err)
	}

	if len(clusterRoleBindingsToDelete) != 0 {
		r.logger.Debugf(ctx, "deleting the cluster role bindings in the Kubernetes API")

		// Delete the cluster role bindings in the Kubernetes API.
		for _, clusterRoleBinding := range clusterRoleBindingsToDelete {
			err := r.k8sClient.RbacV1beta1().ClusterRoleBindings().Delete(ctx, clusterRoleBinding.Name, metav1.DeleteOptions{})
			if apierrors.IsNotFound(err) {
			} else if err != nil {
				return microerror.Mask(err)
			}
		}

		r.logger.Debugf(ctx, "deleted the cluster role bindings in the Kubernetes API")
	} else {
		r.logger.Debugf(ctx, "the cluster role bindings do not need to be deleted from the Kubernetes API")
	}

	return nil
}

func (r *Resource) NewDeletePatch(ctx context.Context, obj, currentState, desiredState interface{}) (*crud.Patch, error) {
	delete, err := r.newDeleteChangeForDeletePatch(ctx, obj, currentState, desiredState)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	patch := crud.NewPatch()
	patch.SetDeleteChange(delete)

	return patch, nil
}

func (r *Resource) newDeleteChangeForDeletePatch(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, error) {
	currentClusterRoleBindings, err := toClusterRoleBindings(currentState)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	desiredClusterRoleBindings, err := toClusterRoleBindings(desiredState)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	r.logger.Debugf(ctx, "finding out which cluster role bindings have to be deleted")

	var clusterRoleBindingsToDelete []*apiv1.ClusterRoleBinding

	for _, currentClusterRoleBinding := range currentClusterRoleBindings {
		if containsClusterRoleBinding(desiredClusterRoleBindings, currentClusterRoleBinding) {
			clusterRoleBindingsToDelete = append(clusterRoleBindingsToDelete, currentClusterRoleBinding)
		}
	}

	r.logger.Debugf(ctx, "found %d cluster role bindings that have to be deleted", len(clusterRoleBindingsToDelete))

	return clusterRoleBindingsToDelete, nil
}
