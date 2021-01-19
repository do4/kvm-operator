package deployment

import (
	"fmt"

	"github.com/giantswarm/apiextensions/v3/pkg/apis/infrastructure/v1alpha2"
	"github.com/giantswarm/apiextensions/v3/pkg/apis/provider/v1alpha1"
	releasev1alpha1 "github.com/giantswarm/apiextensions/v3/pkg/apis/release/v1alpha1"
	"github.com/giantswarm/microerror"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/kvm-operator/pkg/label"
	"github.com/giantswarm/kvm-operator/pkg/project"
	"github.com/giantswarm/kvm-operator/service/controller/key"
)

func newMasterDeployment(cluster v1alpha2.KVMCluster, release releasev1alpha1.Release, i int, masterNode v1alpha1.ClusterNode) (*v1.Deployment, error) {
	privileged := true
	replicas := int32(1)
	podDeletionGracePeriod := int64(key.PodDeletionGracePeriod.Seconds())

	containerDistroVersion, err := key.ContainerDistro(release)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	capabilities := cluster.Spec.KVM.Masters[i]

	cpuQuantity, err := key.CPUQuantity(capabilities)
	if err != nil {
		return nil, microerror.Maskf(invalidConfigError, "error creating CPU quantity: %s", err)
	}

	memoryQuantity, err := key.MemoryQuantityMaster(capabilities)
	if err != nil {
		return nil, microerror.Maskf(invalidConfigError, "error creating memory quantity: %s", err)
	}

	storageType := key.StorageType(cluster)

	// During migration, some TPOs do not have storage type set.
	// This specifies a default, until all TPOs have the correct storage type set.
	// tl;dr - this shouldn't be here. If all TPOs have storageType, remove it.
	if storageType == "" {
		storageType = "hostPath"
	}

	var etcdVolume corev1.Volume
	if storageType == "hostPath" {
		etcdVolume = corev1.Volume{
			Name: "etcd-data",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: key.MasterHostPathVolumeDir(key.ClusterID(cluster), key.VMNumber(i)),
				},
			},
		}
	} else if storageType == "persistentVolume" {
		etcdVolume = corev1.Volume{
			Name: "etcd-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: key.EtcdPVCName(key.ClusterID(cluster), key.VMNumber(i)),
				},
			},
		}
	} else {
		return nil, microerror.Maskf(wrongTypeError, "unknown storageType: '%s'", key.StorageType(cluster))
	}

	deployment := &v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: key.DeploymentName(key.MasterID, masterNode.ID),
			Annotations: map[string]string{
				key.ReleaseVersionAnnotation:       key.ReleaseVersion(cluster),
				key.VersionBundleVersionAnnotation: key.OperatorVersion(cluster),
			},
			Labels: map[string]string{
				key.LabelApp:          key.MasterID,
				key.LabelCluster:      key.ClusterID(cluster),
				key.LabelOrganization: key.ClusterCustomer(cluster),
				key.LabelManagedBy:    key.OperatorName,
				"node":                masterNode.ID,
			},
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					key.LabelApp: key.MasterID,
					"cluster":    key.ClusterID(cluster),
					"node":       masterNode.ID,
				},
			},
			Strategy: v1.DeploymentStrategy{
				Type: v1.RecreateDeploymentStrategyType,
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						key.AnnotationAPIEndpoint:   key.ClusterAPIEndpoint(cluster),
						key.AnnotationIp:            "",
						key.AnnotationService:       key.MasterID,
						key.AnnotationPodDrained:    "False",
						key.AnnotationVersionBundle: key.OperatorVersion(cluster),
					},
					GenerateName: key.MasterID,
					Labels: map[string]string{
						key.LabelApp:          key.MasterID,
						key.LabelCluster:      key.ClusterID(cluster),
						key.LabelOrganization: key.ClusterCustomer(cluster),
						"node":                masterNode.ID,
						key.PodWatcherLabel:   key.OperatorName,
						label.OperatorVersion: project.Version(),
					},
				},
				Spec: corev1.PodSpec{
					Affinity: newMasterPodAfinity(cluster),
					NodeSelector: map[string]string{
						"role": key.MasterID,
					},
					ServiceAccountName:            key.ServiceAccountName(cluster),
					TerminationGracePeriodSeconds: &podDeletionGracePeriod,
					Volumes: []corev1.Volume{
						{
							Name: "ignition-cm",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: key.ConfigMapName(cluster, masterNode, key.MasterID),
									},
								},
							},
						},
						{
							Name: "ignition",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						etcdVolume,
						{
							Name: "images",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: key.FlatcarImageDir,
								},
							},
						},
						{
							Name: "rootfs",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "dev-kvm",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/dev/kvm",
								},
							},
						},
						{
							Name: "dev-net-tun",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/dev/net/tun",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "k8s-kvm",
							Image:           key.K8SKVMDockerImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"NET_ADMIN",
									},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "GUEST_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.name",
										},
									},
								},
								{
									Name: "GUEST_MEMORY",
									// TODO provide memory like disk as float64 and format here.
									Value: capabilities.Memory,
								},
								{
									Name:  "GUEST_CPUS",
									Value: fmt.Sprintf("%d", capabilities.CPUs),
								},
								{
									Name:  "GUEST_ROOT_DISK_SIZE",
									Value: key.DefaultOSDiskSize,
								},
								{
									Name:  "FLATCAR_CHANNEL",
									Value: key.FlatcarChannel,
								},
								{
									Name:  "FLATCAR_VERSION",
									Value: containerDistroVersion,
								},
								{
									Name:  "FLATCAR_IGNITION",
									Value: "/var/lib/containervmm/ignition/ignition",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    cpuQuantity,
									corev1.ResourceMemory: memoryQuantity,
								},
								Limits: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    cpuQuantity,
									corev1.ResourceMemory: memoryQuantity,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "ignition",
									MountPath: "/var/lib/containervmm/ignition",
								},
								{
									Name:      "etcd-data",
									MountPath: "/var/lib/containervmm/etcd",
								},
								{
									Name:      "images",
									MountPath: "/var/lib/containervmm/flatcar",
								},
								{
									Name:      "rootfs",
									MountPath: "/var/lib/containervmm/rootfs",
								},
								{
									Name:      "dev-kvm",
									MountPath: "/dev/kvm",
								},
								{
									Name:      "dev-net-tun",
									MountPath: "/dev/net/tun",
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:  "ignition",
							Image: key.K8SKVMDockerImage,
							Command: []string{
								"cp",
								"/tmp/ignition/user_data",
								"/var/lib/containervmm/ignition/ignition",
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "ignition",
									MountPath: "/var/lib/containervmm/ignition",
								},
								{
									Name:      "ignition-cm",
									MountPath: "/tmp/ignition",
								},
							},
						},
					},
				},
			},
		},
	}

	addCoreComponentsAnnotations(deployment, release)

	return deployment, nil
}

func newMasterPodAfinity(cluster v1alpha2.KVMCluster) *corev1.Affinity {
	podAffinity := &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "app",
								Operator: metav1.LabelSelectorOpIn,
								Values: []string{
									"master",
									"worker",
								},
							},
						},
					},
					TopologyKey: "kubernetes.io/hostname",
					Namespaces: []string{
						key.ClusterID(cluster),
					},
				},
			},
		},
	}

	return podAffinity
}
