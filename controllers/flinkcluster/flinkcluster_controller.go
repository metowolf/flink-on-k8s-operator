/*
Copyright 2019 Google LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package flinkcluster

import (
	"context"
	"time"

	"github.com/spotify/flink-on-k8s-operator/internal/controllers/history"
	"github.com/spotify/flink-on-k8s-operator/internal/flink"

	"github.com/go-logr/logr"
	v1beta1 "github.com/spotify/flink-on-k8s-operator/apis/flinkcluster/v1beta1"
	"github.com/spotify/flink-on-k8s-operator/internal/model"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = v1beta1.GroupVersion.WithKind("FlinkCluster")

// FlinkClusterReconciler reconciles a FlinkCluster object
type FlinkClusterReconciler struct {
	Client        client.Client
	Clientset     *kubernetes.Clientset
	EventRecorder record.EventRecorder
}

func NewReconciler(mgr manager.Manager) (*FlinkClusterReconciler, error) {
	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	return &FlinkClusterReconciler{
		Client:        mgr.GetClient(),
		Clientset:     cs,
		EventRecorder: mgr.GetEventRecorderFor("FlinkOperator"),
	}, nil
}

// +kubebuilder:rbac:groups=flinkoperator.k8s.io,resources=flinkclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=flinkoperator.k8s.io,resources=flinkclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups=networking,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking,resources=ingresses/status,verbs=get

// Reconcile the observed state towards the desired state for a FlinkCluster custom resource.
func (r *FlinkClusterReconciler) Reconcile(ctx context.Context,
	request ctrl.Request) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx)

	var handler = FlinkClusterHandler{
		k8sClient:     r.Client,
		k8sClientset:  r.Clientset,
		flinkClient:   flink.NewDefaultClient(log),
		request:       request,
		eventRecorder: r.EventRecorder,
		observed:      ObservedClusterState{},
	}

	return handler.reconcile(logr.NewContext(ctx, log), request)
}

// SetupWithManager registers this reconciler with the controller manager and
// starts watching FlinkCluster, Deployment and Service resources.
func (reconciler *FlinkClusterReconciler) SetupWithManager(
	mgr ctrl.Manager,
	maxConcurrentReconciles int) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(ctrlcontroller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		For(&v1beta1.FlinkCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&batchv1.Job{}).
		Complete(reconciler)
}

// FlinkClusterHandler holds the context and state for a
// reconcile request.
type FlinkClusterHandler struct {
	k8sClient     client.Client
	k8sClientset  *kubernetes.Clientset
	flinkClient   *flink.Client
	request       ctrl.Request
	eventRecorder record.EventRecorder
	observed      ObservedClusterState
	desired       model.DesiredClusterState
}

func (handler *FlinkClusterHandler) reconcile(ctx context.Context,
	request ctrl.Request) (ctrl.Result, error) {
	var k8sClient = handler.k8sClient
	var flinkClient = handler.flinkClient
	var log = logr.FromContextOrDiscard(ctx)
	var observed = &handler.observed
	var desired = &handler.desired
	var statusChanged bool
	var err error

	// History interface
	var history = history.NewHistory(k8sClient, ctx)

	log.Info("============================================================")
	log.Info("---------- 1. Observe the current state ----------")

	var observer = ClusterStateObserver{
		k8sClient:    k8sClient,
		k8sClientset: handler.k8sClientset,
		flinkClient:  flinkClient,
		request:      request,
		recorder:     handler.eventRecorder,
		history:      history,
	}
	err = observer.observe(ctx, observed)
	if err != nil {
		log.Error(err, "Failed to observe the current state")
		return ctrl.Result{}, err
	}

	// Sync history and observe revision status
	err = observer.syncRevisionStatus(observed)
	if err != nil {
		log.Error(err, "Failed to sync flinkCluster history")
		return ctrl.Result{}, err
	}

	log.Info("---------- 2. Update cluster status ----------")

	var updater = ClusterStatusUpdater{
		k8sClient: k8sClient,
		recorder:  handler.eventRecorder,
		observed:  handler.observed,
	}
	statusChanged, err = updater.updateStatusIfChanged(ctx)
	if err != nil {
		log.Error(err, "Failed to update cluster status")
		return ctrl.Result{}, err
	}
	if statusChanged {
		log.Info(
			"Wait status to be stable before taking further actions.",
			"requeueAfter",
			5)
		return ctrl.Result{
			Requeue: true, RequeueAfter: 5 * time.Second,
		}, nil
	}

	log.Info("---------- 3. Compute the desired state ----------")

	*desired = *getDesiredClusterState(observed)
	if desired.ConfigMap != nil {
		log = log.WithValues("ConfigMap", *desired.ConfigMap)
	} else {
		log = log.WithValues("ConfigMap", "nil")
	}
	if desired.PodDisruptionBudget != nil {
		log = log.WithValues("PodDisruptionBudget", *desired.PodDisruptionBudget)
	} else {
		log = log.WithValues("PodDisruptionBudget", "nil")
	}
	if desired.TmService != nil {
		log = log.WithValues("TaskManager Service", *desired.TmService)
	} else {
		log = log.WithValues("TaskManager Service", "nil")
	}
	if desired.JmStatefulSet != nil {
		log = log.WithValues("JobManager StatefulSet", *desired.JmStatefulSet)
	} else {
		log = log.WithValues("JobManager StatefulSet", "nil")
	}
	if desired.JmService != nil {
		log = log.WithValues("JobManager service", *desired.JmService)
	} else {
		log = log.WithValues("JobManager service", "nil")
	}
	if desired.JmIngress != nil {
		log = log.WithValues("JobManager ingress", *desired.JmIngress)
	} else {
		log = log.WithValues("JobManager ingress", "nil")
	}
	if desired.TmStatefulSet != nil {
		log = log.WithValues("TaskManager StatefulSet", *desired.TmStatefulSet)
	} else if desired.TmDeployment != nil {
		log = log.WithValues("TaskManager Deployment", *desired.TmDeployment)
	} else {
		log = log.WithValues("TaskManager", "nil")
	}
	if desired.HorizontalPodAutoscaler != nil {
		log = log.WithValues("HorizontalPodAutoscaler", *desired.HorizontalPodAutoscaler)
	} else {
		log = log.WithValues("HorizontalPodAutoscaler", "nil")
	}

	if desired.Job != nil {
		log = log.WithValues("Job", *desired.Job)
	} else {
		log = log.WithValues("Job", "nil")
	}
	log.Info("Desired state")

	log.Info("---------- 4. Take actions ----------")

	var reconciler = ClusterReconciler{
		k8sClient:   k8sClient,
		flinkClient: flinkClient,
		observed:    handler.observed,
		desired:     handler.desired,
		recorder:    handler.eventRecorder,
	}
	result, err := reconciler.reconcile(ctx)
	if err != nil {
		log.Error(err, "Failed to reconcile")
	}
	if result.RequeueAfter > 0 {
		log.Info("Requeue reconcile request", "after", result.RequeueAfter)
	}

	return result, err
}
