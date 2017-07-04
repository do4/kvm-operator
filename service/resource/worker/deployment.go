package worker

import (
	"fmt"
	"path/filepath"

	"github.com/giantswarm/kvm-operator/service/resource"
	"github.com/giantswarm/kvmtpr"
	microerror "github.com/giantswarm/microkit/error"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	extensionsv1 "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

func (s *Service) newDeployments(obj interface{}) ([]*extensionsv1.Deployment, error) {
	var deployments []*extensionsv1.Deployment

	customObject, ok := obj.(*kvmtpr.CustomObject)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &kvmtpr.CustomObject{}, obj)
	}

	privileged := true
	replicas := int32(1)

	for i, workerNode := range customObject.Spec.Cluster.Workers {
		capabilities := customObject.Spec.KVM.Workers[i]

		deployment := &extensionsv1.Deployment{
			TypeMeta: apismetav1.TypeMeta{
				Kind:       "deployment",
				APIVersion: "extensions/v1beta",
			},
			ObjectMeta: apismetav1.ObjectMeta{
				Name: "worker-" + workerNode.ID,
				Labels: map[string]string{
					"cluster":  resource.ClusterID(*customObject),
					"customer": resource.ClusterCustomer(*customObject),
					"app":      "worker",
					"node":     workerNode.ID,
				},
			},
			Spec: extensionsv1.DeploymentSpec{
				Strategy: extensionsv1.DeploymentStrategy{
					Type: extensionsv1.RecreateDeploymentStrategyType,
				},
				Replicas: &replicas,
				Template: apiv1.PodTemplateSpec{
					ObjectMeta: apismetav1.ObjectMeta{
						Name: "worker",
						Labels: map[string]string{
							"cluster":  resource.ClusterID(*customObject),
							"customer": resource.ClusterCustomer(*customObject),
							"app":      "worker",
							"node":     workerNode.ID,
						},
						Annotations: map[string]string{},
					},
					Spec: apiv1.PodSpec{
						HostNetwork: true,
						Volumes: []apiv1.Volume{
							{
								Name: "cloud-config",
								VolumeSource: apiv1.VolumeSource{
									ConfigMap: &apiv1.ConfigMapVolumeSource{
										LocalObjectReference: apiv1.LocalObjectReference{
											Name: resource.ConfigMapName(*customObject, workerNode, "worker"),
										},
									},
								},
							},
							{
								Name: "images",
								VolumeSource: apiv1.VolumeSource{
									HostPath: &apiv1.HostPathVolumeSource{
										Path: "/home/core/images/",
									},
								},
							},
							{
								Name: "rootfs",
								VolumeSource: apiv1.VolumeSource{
									HostPath: &apiv1.HostPathVolumeSource{
										Path: filepath.Join("/home/core/vms", resource.ClusterID(*customObject), workerNode.ID),
									},
								},
							},
						},
						Containers: []apiv1.Container{
							{
								Name:            "k8s-endpoint-updater",
								Image:           customObject.Spec.KVM.EndpointUpdater.Docker.Image,
								ImagePullPolicy: apiv1.PullIfNotPresent,
								Command: []string{
									"/bin/sh",
									"-c",
									"/opt/k8s-endpoint-updater update --provider.bridge.name=${NETWORK_BRIDGE_NAME} --provider.kind=bridge --service.kubernetes.address=\"\" --service.kubernetes.cluster.namespace=${POD_NAMESPACE} --service.kubernetes.cluster.service=worker --service.kubernetes.inCluster=true --updater.pod.names=${POD_NAME}; while true; do sleep inf; done",
								},
								SecurityContext: &apiv1.SecurityContext{
									Privileged: &privileged,
								},
								Env: []apiv1.EnvVar{
									{
										Name:  "NETWORK_BRIDGE_NAME",
										Value: resource.NetworkBridgeName(resource.ClusterID(*customObject)),
									},
									{
										Name: "POD_NAME",
										ValueFrom: &apiv1.EnvVarSource{
											FieldRef: &apiv1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "metadata.name",
											},
										},
									},
									{
										Name: "POD_NAMESPACE",
										ValueFrom: &apiv1.EnvVarSource{
											FieldRef: &apiv1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "metadata.namespace",
											},
										},
									},
								},
							},
							{
								Name:            "k8s-kvm",
								Image:           customObject.Spec.KVM.K8sKVM.Docker.Image,
								ImagePullPolicy: apiv1.PullIfNotPresent,
								SecurityContext: &apiv1.SecurityContext{
									Privileged: &privileged,
								},
								Args: []string{
									"worker",
								},
								Env: []apiv1.EnvVar{
									{
										Name:  "CORES",
										Value: fmt.Sprintf("%d", capabilities.CPUs),
									},
									{
										Name:  "DISK",
										Value: fmt.Sprintf("%.0fG", capabilities.Disk),
									},
									{
										Name: "HOSTNAME",
										ValueFrom: &apiv1.EnvVarSource{
											FieldRef: &apiv1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "metadata.name",
											},
										},
									},
									{
										Name:  "NETWORK_BRIDGE_NAME",
										Value: resource.NetworkBridgeName(resource.ClusterID(*customObject)),
									},
									{
										Name: "MEMORY",
										// TODO provide memory like disk as float64 and format here.
										Value: capabilities.Memory,
									},
									{
										Name:  "ROLE",
										Value: "worker",
									},
									{
										Name:  "CLOUD_CONFIG_PATH",
										Value: "/cloudconfig/user_data",
									},
								},
								VolumeMounts: []apiv1.VolumeMount{
									{
										Name:      "cloud-config",
										MountPath: "/cloudconfig/",
									},
									{
										Name:      "images",
										MountPath: "/usr/code/images/",
									},
									{
										Name:      "rootfs",
										MountPath: "/usr/code/rootfs/",
									},
								},
							},
						},
					},
				},
			},
		}

		deployments = append(deployments, deployment)
	}

	return deployments, nil
}