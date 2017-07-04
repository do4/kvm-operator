package k8s

import (
	"fmt"

	microerror "github.com/giantswarm/microkit/error"
	micrologger "github.com/giantswarm/microkit/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	apisbatchv1 "k8s.io/client-go/pkg/apis/batch/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

// Config represents the configuration used to create a new reconciler.
type Config struct {
	// Dependencies.
	KubernetesClient *kubernetes.Clientset
	ListDecoder      ListDecoder
	Logger           micrologger.Logger
	WatchDecoder     WatchDecoder

	// Settings.
	ListEndpoint string
	// Resources is the list of resources to be processed during creation and
	// deletion reconciliations. The current reconciliation is synchronous and
	// processes resources in a series. One resource after another will be
	// processed.
	Resources     []Resource
	WatchEndpoint string
}

// DefaultConfig provides a default configuration to create a new reconciler by
// best effort.
func DefaultConfig() Config {
	return Config{
		// Dependencies.
		KubernetesClient: nil,
		ListDecoder:      nil,
		Logger:           nil,
		WatchDecoder:     &watchDecoder{},

		// Settings.
		ListEndpoint:  "",
		Resources:     nil,
		WatchEndpoint: "",
	}
}

// New creates a new configured reconciler.
func New(config Config) (*Reconciler, error) {
	// Dependencies.
	if config.KubernetesClient == nil {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.KubernetesClient must not be empty")
	}
	if config.ListDecoder == nil {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.ListDecoder must not be empty")
	}
	if config.Logger == nil {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.Logger must not be empty")
	}
	if config.WatchDecoder == nil {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.WatchDecoder must not be empty")
	}

	// Settings.
	if config.ListEndpoint == "" {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.ListEndpoint must not be empty")
	}
	if len(config.Resources) == 0 {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.Resources must not be empty")
	}
	if config.WatchEndpoint == "" {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.WatchEndpoint must not be empty")
	}

	newReconciler := &Reconciler{
		// Dependencies.
		kubernetesClient: config.KubernetesClient,
		listDecoder:      config.ListDecoder,
		logger:           config.Logger,
		watchDecoder:     config.WatchDecoder,

		// Settings
		listEndpoint:  config.ListEndpoint,
		resources:     config.Resources,
		watchEndpoint: config.WatchEndpoint,
	}

	return newReconciler, nil
}

// Reconciler implements the reconciler.
type Reconciler struct {
	// Dependencies.
	kubernetesClient *kubernetes.Clientset
	listDecoder      ListDecoder
	logger           micrologger.Logger
	watchDecoder     WatchDecoder

	// Settings.
	listEndpoint  string
	resources     []Resource
	watchEndpoint string
}

// GetAddFunc returns the add function used to be registered in Kubernetes
// client watches.
func (r *Reconciler) GetAddFunc() func(obj interface{}) {
	return func(obj interface{}) {
		r.logger.Log("debug", "executing the reconciler's add function", "event", "create")

		var runtimeObjects []runtime.Object
		var namespace *v1.Namespace

		for _, res := range r.resources {
			ros, err := res.GetForCreate(obj)
			if err != nil {
				r.logger.Log("error", err.Error(), "event", "create")
			}

			for _, ro := range ros {
				switch t := ro.(type) {
				case *v1.Namespace:
					namespace = t
				}
			}

			runtimeObjects = append(runtimeObjects, ros...)
		}

		if namespace == nil {
			r.logger.Log("error", "namespace must not be empty", "event", "create")
			return
		}

		for _, ro := range runtimeObjects {
			var err error

			switch t := ro.(type) {
			case *v1.ConfigMap:
				_, err = r.kubernetesClient.Core().ConfigMaps(namespace.Name).Create(t)
			case *v1beta1.Deployment:
				_, err = r.kubernetesClient.Extensions().Deployments(namespace.Name).Create(t)
			case *v1beta1.Ingress:
				_, err = r.kubernetesClient.Extensions().Ingresses(namespace.Name).Create(t)
			case *apisbatchv1.Job:
				_, err = r.kubernetesClient.BatchV1().Jobs(namespace.Name).Create(t)
			case *v1.Namespace:
				_, err = r.kubernetesClient.Core().Namespaces().Create(t)
			case *v1.Service:
				_, err = r.kubernetesClient.Core().Services(namespace.Name).Create(t)
			default:
				r.logger.Log("error", fmt.Sprintf("unknown runtime object type '%T'", t), "event", "create")
			}

			if apierrors.IsAlreadyExists(err) {
				// Resource already being created, all good.
			} else if err != nil {
				r.logger.Log("error", err.Error(), "event", "create")
			}
		}
	}
}

// GetDeleteFunc returns the delete function used to be registered in Kubernetes
// client watches. The reconcilliation collects all runtime objects based on the
// configured resources before applying the delete operations on them. In case
// the runtime object is nil, we do not track it, that is, there will be no
// deletion being performed on the configured resource. The client's intention
// is then to not process any deletion action, so this implementation detail is
// up to the client's responsibility. This might make sense if the only resource
// intended to be deleted is the Kubernetes namespace, which in turn deletes all
// other resources being inside this namespace.
func (r *Reconciler) GetDeleteFunc() func(obj interface{}) {
	return func(obj interface{}) {
		r.logger.Log("debug", "executing the reconciler's delete function", "event", "delete")

		var runtimeObjects []runtime.Object
		var namespace *v1.Namespace

		for _, res := range r.resources {
			ros, err := res.GetForDelete(obj)
			if err != nil {
				r.logger.Log("error", err.Error(), "event", "delete")
				return
			}

			for _, ro := range ros {
				switch t := ro.(type) {
				case *v1.Namespace:
					namespace = t
				}
			}

			runtimeObjects = append(runtimeObjects, ros...)
		}

		if namespace == nil {
			r.logger.Log("error", "namespace must not be empty", "event", "delete")
			return
		}

		for _, ro := range runtimeObjects {
			var err error

			switch t := ro.(type) {
			case *v1.ConfigMap:
				err = r.kubernetesClient.Core().ConfigMaps(namespace.Name).Delete(t.Name, nil)
			case *v1beta1.Deployment:
				err = r.kubernetesClient.Extensions().Deployments(namespace.Name).Delete(t.Name, nil)
			case *v1beta1.Ingress:
				err = r.kubernetesClient.Extensions().Ingresses(namespace.Name).Delete(t.Name, nil)
			case *apisbatchv1.Job:
				err = r.kubernetesClient.BatchV1().Jobs(namespace.Name).Delete(t.Name, nil)
			case *v1.Namespace:
				err = r.kubernetesClient.Core().Namespaces().Delete(t.Name, nil)
			case *v1.Service:
				err = r.kubernetesClient.Core().Services(namespace.Name).Delete(t.Name, nil)
			default:
				r.logger.Log("error", fmt.Sprintf("unknown runtime object type '%T'", t), "event", "delete")
			}

			if apierrors.IsNotFound(err) {
				// Resource already being deleted, all good.
			} else if err != nil {
				r.logger.Log("error", err.Error(), "event", "delete")
			}
		}
	}
}

// GetListWatch returns the list-watch used to be registered in Kubernetes
// client watches.
func (r *Reconciler) GetListWatch() *cache.ListWatch {
	listWatch := &cache.ListWatch{
		ListFunc: func(options apismetav1.ListOptions) (runtime.Object, error) {
			r.logger.Log("debug", "executing the reconciler's list function", "event", "list")

			req := r.kubernetesClient.Core().RESTClient().Get().AbsPath(r.listEndpoint)
			b, err := req.DoRaw()
			if err != nil {
				return nil, microerror.MaskAny(err)
			}

			v, err := r.listDecoder.Decode(b)
			if err != nil {
				return nil, microerror.MaskAny(err)
			}

			return v, nil
		},
		WatchFunc: func(options apismetav1.ListOptions) (watch.Interface, error) {
			r.logger.Log("debug", "executing Extensionsthe reconciler's watch function", "event", "watch")

			req := r.kubernetesClient.Core().RESTClient().Get().AbsPath(r.watchEndpoint)
			stream, err := req.Stream()
			if err != nil {
				return nil, microerror.MaskAny(err)
			}

			r.watchDecoder.SetStream(stream)

			return watch.NewStreamWatcher(r.watchDecoder), nil
		},
	}

	return listWatch
}