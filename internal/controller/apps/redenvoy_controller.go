/*
Copyright 2026 Abstract Prism.

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

package apps

import (
	"context"
	redenvoy "red-envoy/api/apps/v1"

	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// RedEnvoyReconciler reconciles a RedEnvoy object
type RedEnvoyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.abstractprism.com,resources=redenvoys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.abstractprism.com,resources=redenvoys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.abstractprism.com,resources=redenvoys/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps.abstractprism.com,resources=redenvoys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.abstractprism.com,resources=redenvoys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.abstractprism.com,resources=redenvoys/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete;bind

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RedEnvoy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/reconcile
func (r *RedEnvoyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling RedEnvoy", "namespace", req.Namespace, "name", req.Name)

	var redenvoy redenvoy.RedEnvoy
	if err := r.Get(ctx, req.NamespacedName, &redenvoy); err != nil {
		if errors.IsNotFound(err) {
			log.Info("RedEnvoy resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get RedEnvoy")
		return ctrl.Result{}, err
	}

	// --- Web Server Infrastructure ---
	// 1. ServiceAccount
	sa := r.serviceAccountForWebServer(&redenvoy)
	foundSA := &corev1.ServiceAccount{}
	if err := r.Get(ctx, types.NamespacedName{Name: sa.Name, Namespace: sa.Namespace}, foundSA); err != nil && errors.IsNotFound(err) {
		log.Info("Creating Web Server ServiceAccount", "Name", sa.Name)
		if err := r.Create(ctx, sa); err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// 2. Role
	role := r.roleForWebServer(&redenvoy)
	foundRole := &rbacv1.Role{}
	if err := r.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, foundRole); err != nil && errors.IsNotFound(err) {
		log.Info("Creating Web Server Role", "Name", role.Name)
		if err := r.Create(ctx, role); err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// 3. RoleBinding
	rb := r.roleBindingForWebServer(&redenvoy)
	foundRB := &rbacv1.RoleBinding{}
	if err := r.Get(ctx, types.NamespacedName{Name: rb.Name, Namespace: rb.Namespace}, foundRB); err != nil && errors.IsNotFound(err) {
		log.Info("Creating Web Server RoleBinding", "Name", rb.Name)
		if err := r.Create(ctx, rb); err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// 4. Deployment
	deploy := r.deploymentForWebServer(&redenvoy)
	foundDeploy := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, foundDeploy); err != nil && errors.IsNotFound(err) {
		log.Info("Creating Web Server Deployment", "Name", deploy.Name)
		if err := r.Create(ctx, deploy); err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// 5. Service
	svc := r.serviceForWebServer(&redenvoy)
	foundSvc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, foundSvc); err != nil && errors.IsNotFound(err) {
		log.Info("Creating Web Server Service", "Name", svc.Name)
		if err := r.Create(ctx, svc); err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Check for the pod creation annotation
	// podNameToCreate, ok := redenvoy.Annotations[podCreateAnnotation]
	// if !ok {
	// 	// No pod creation request, so nothing to do.
	// 	log.Info("No pod creation request found in annotations.")
	// 	return ctrl.Result{}, nil
	// }
	// --- Garbage Collection ---
	// Create a map of desired pods for efficient lookup
	desiredPods := make(map[string]bool)
	for _, podName := range redenvoy.Status.ManagedPods {
		desiredPods[podName] = true
	}

	//log.Info("Found pod creation request in annotation", "podName", podNameToCreate)
	// List and check Pods
	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(req.Namespace), client.MatchingLabels{"app": redenvoy.Name}); err != nil {
		log.Error(err, "unable to list child Pods")
		return ctrl.Result{}, err
	}

	// Create the pod
	for _, pod := range podList.Items {
		if pod.GetLabels()["component"] == "operator-webhook" {
			continue
		}

		if _, shouldExist := desiredPods[pod.Name]; !shouldExist {
			log.Info("Deleting orphaned Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
			if err := r.Delete(ctx, &pod); err != nil {
				log.Error(err, "unable to delete orphaned Pod")
				return ctrl.Result{}, err
			}
		}
	}

	// List and check Services
	var serviceList corev1.ServiceList
	if err := r.List(ctx, &serviceList, client.InNamespace(req.Namespace), client.MatchingLabels{"app": redenvoy.Name}); err != nil {
		log.Error(err, "unable to list child Services")
		return ctrl.Result{}, err
	}

	for _, service := range serviceList.Items {
		if service.GetLabels()["component"] == "operator-webhook" {
			continue
		}
		if _, shouldExist := desiredPods[service.Name]; !shouldExist {
			log.Info("Deleting orphaned Service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
			if err := r.Delete(ctx, &service); err != nil {
				log.Error(err, "unable to delete orphaned Service")
				return ctrl.Result{}, err
			}
		}
	}

	// List and check Ingresses
	var ingressList networkingv1.IngressList
	// Note: This requires that ingresses have the "app" label.
	if err := r.List(ctx, &ingressList, client.InNamespace(req.Namespace), client.MatchingLabels{"app": redenvoy.Name}); err != nil {
		log.Error(err, "unable to list child Ingresses")
		return ctrl.Result{}, err
	}

	// Create the ingress only if IngressHost is specified
	// if redenvoy.Spec.IngressHost != "" {
	// 	ingress := r.ingressForRedEnvoyApp(&redenvoy, podNameToCreate)
	// 	foundIngress := &networkingv1.Ingress{}
	// 	err = r.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, foundIngress)
	for _, ingress := range ingressList.Items {
		if ingress.GetLabels()["component"] == "operator-webhook" {
			continue
		}
		if _, shouldExist := desiredPods[ingress.Name]; !shouldExist {
			log.Info("Deleting orphaned Ingress", "Ingress.Namespace", ingress.Namespace, "Ingress.Name", ingress.Name)
			if err := r.Delete(ctx, &ingress); err != nil {
				log.Error(err, "unable to delete orphaned Ingress")
				return ctrl.Result{}, err
			}
		}
	}

	// --- Creation/Update Loop ---
	for podNameToCreate := range desiredPods {
		// Create the pod if it doesn't exist
		pod := r.podForRedEnvoyApp(&redenvoy, podNameToCreate)
		foundPod := &corev1.Pod{}
		err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, foundPod)
		if err != nil && errors.IsNotFound(err) {
			log.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
			if err := r.Create(ctx, pod); err != nil {
				log.Error(err, "unable to create Pod")
				return ctrl.Result{}, err
			}
		} else if err != nil {
			log.Error(err, "failed to get Ingress")
			return ctrl.Result{}, err
		}

		// Remove the annotation to signify we have processed it
		// Create the service if it doesn't exist
		service := r.serviceForRedEnvoyApp(&redenvoy, podNameToCreate)
		foundService := &corev1.Service{}
		err = r.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, foundService)
		if err != nil && errors.IsNotFound(err) {
			log.Info("Creating a new Service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
			if err := r.Create(ctx, service); err != nil {
				log.Error(err, "unable to create Service")
				return ctrl.Result{}, err
			}
		} else if err != nil {
			log.Error(err, "failed to get Service")
			return ctrl.Result{}, err
		}

		// Create the ingress only if IngressHost is specified and it doesn't exist
		if redenvoy.Spec.IngressHost != "" {
			ingress := r.ingressForRedEnvoyApp(&redenvoy, podNameToCreate)
			foundIngress := &networkingv1.Ingress{}
			err = r.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, foundIngress)
			if err != nil && errors.IsNotFound(err) {
				log.Info("Creating a new Ingress", "Ingress.Namespace", ingress.Namespace, "Ingress.Name", ingress.Name)
				if err := r.Create(ctx, ingress); err != nil {
					log.Error(err, "unable to create Ingress")
					return ctrl.Result{}, err
				}
			} else if err != nil {
				log.Error(err, "failed to get Ingress")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *RedEnvoyReconciler) mapEventToRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)
	event, ok := obj.(*corev1.Event)
	if !ok {
		return nil
	}

	// handles events only for RedEnvoy objects
	if event.InvolvedObject.Kind != "RedEnvoy" {
		return nil
	}

	var redenvoy redenvoy.RedEnvoy
	err := r.Get(ctx, types.NamespacedName{Name: event.InvolvedObject.Name, Namespace: event.InvolvedObject.Namespace}, &redenvoy)
	if err != nil {
		log.Error(err, "failed to get involved RedEnvoy from event")
		return nil
	}

	// Check if the event reason matches what the RedEnvoy is waiting for.
	podName := event.Message
	statusNeedsUpdate := false

	// Check for creation event
	if redenvoy.Spec.TriggeredEventName != "" && redenvoy.Spec.TriggeredEventName == event.Reason {
		log.Info("Found matching event for RedEnvoy", "redenvoy", redenvoy.Name, "eventReason", event.Reason, "podNameFromMessage", event.Message)
		// Add to managed pods if not already present
		found := false
		for _, name := range redenvoy.Status.ManagedPods {
			if name == podName {
				found = true
				break
			}
		}
		if !found {
			redenvoy.Status.ManagedPods = append(redenvoy.Status.ManagedPods, podName)
			statusNeedsUpdate = true
		}
	}

	// Check for deletion event
	if redenvoy.Spec.TriggeredDeleteEventName != "" && redenvoy.Spec.TriggeredDeleteEventName == event.Reason {
		log.Info("Found matching deletion event for RedEnvoy", "redenvoy", redenvoy.Name, "podName", podName)

		// Remove from managed pods if present
		newManagedPods := []string{}
		for _, name := range redenvoy.Status.ManagedPods {
			if name != podName {
				newManagedPods = append(newManagedPods, name)
			}
		}
		if len(newManagedPods) != len(redenvoy.Status.ManagedPods) {
			redenvoy.Status.ManagedPods = newManagedPods
			statusNeedsUpdate = true
		}
	}

	if statusNeedsUpdate {
		if err := r.Status().Update(ctx, &redenvoy); err != nil {
			log.Error(err, "failed to update RedEnvoy status")
		}
	}
	// The status update will trigger a reconciliation, so we don't need to return a request
	return nil
}
func (r *RedEnvoyReconciler) podForRedEnvoyApp(app *redenvoy.RedEnvoy, podName string) *corev1.Pod {
	labels := map[string]string{
		"app": app.Name,
		"pod": podName,
	}

	var containerPort int32 = 8080 // Default port
	if app.Spec.ContainerPort != nil {
		containerPort = *app.Spec.ContainerPort
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: app.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, redenvoy.GroupVersion.WithKind("RedEnvoy")),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "application",
					Image: app.Spec.ContainerImage,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: containerPort,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
		},
	}
}

// serviceForRedEnvoyApp returns a Service object for the given RedEnvoy and pod name
func (r *RedEnvoyReconciler) serviceForRedEnvoyApp(app *redenvoy.RedEnvoy, podName string) *corev1.Service {
	serviceName := podName // Service name is the same as the pod name
	labels := map[string]string{"app": app.Name}
	selector := map[string]string{
		"app": app.Name,
		"pod": podName,
	}

	var targetPort int32 = 8080 // Default port
	if app.Spec.ContainerPort != nil {
		targetPort = *app.Spec.ContainerPort
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: app.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, redenvoy.GroupVersion.WithKind("RedEnvoy")),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       1880,
					TargetPort: intstr.FromInt(int(targetPort)),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

// ingressForRedEnvoyApp returns an Ingress object for the given RedEnvoy and pod name
func (r *RedEnvoyReconciler) ingressForRedEnvoyApp(app *redenvoy.RedEnvoy, podName string) *networkingv1.Ingress {
	ingressName := podName // Ingress name is the same as the pod name
	serviceName := podName
	path := "/" + podName
	labels := map[string]string{"app": app.Name}
	pathType := networkingv1.PathTypePrefix

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: app.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, redenvoy.GroupVersion.WithKind("RedEnvoy")),
			},
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: app.Spec.IngressHost,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     path,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: serviceName,
											Port: networkingv1.ServiceBackendPort{
												Number: 1880,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *RedEnvoyReconciler) serviceAccountForWebServer(app *redenvoy.RedEnvoy) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name + "-operator-webhook",
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, redenvoy.GroupVersion.WithKind("RedEnvoy")),
			},
		},
	}
}

func (r *RedEnvoyReconciler) roleForWebServer(app *redenvoy.RedEnvoy) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name + "-operator-webhook-role",
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, redenvoy.GroupVersion.WithKind("RedEnvoy")),
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch", "update"},
			},
		},
	}
}

func (r *RedEnvoyReconciler) roleBindingForWebServer(app *redenvoy.RedEnvoy) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name + "-operator-webhook-rolebinding",
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, redenvoy.GroupVersion.WithKind("RedEnvoy")),
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      app.Name + "-operator-webhook",
				Namespace: app.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     app.Name + "-operator-webhook-role",
		},
	}
}

func (r *RedEnvoyReconciler) deploymentForWebServer(app *redenvoy.RedEnvoy) *appsv1.Deployment {
	labels := map[string]string{"app": app.Name, "component": "operator-webhook"}
	replicas := int32(1)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name + "-operator-webhook",
			Namespace: app.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, redenvoy.GroupVersion.WithKind("RedEnvoy")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName: app.Name + "-operator-webhook",
					Containers: []corev1.Container{
						{
							Name:            "webserver",
							Image:           "red-envoy-webhook:latest", // The container image to build below
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports:           []corev1.ContainerPort{{ContainerPort: 8080}},
						},
					},
				},
			},
		},
	}
}

func (r *RedEnvoyReconciler) serviceForWebServer(app *redenvoy.RedEnvoy) *corev1.Service {
	labels := map[string]string{"app": app.Name, "component": "operator-webhook"}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name + "-operator-webhook",
			Namespace: app.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, redenvoy.GroupVersion.WithKind("RedEnvoy")),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedEnvoyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redenvoy.RedEnvoy{}).
		Named("apps-redenvoy").
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Watches(
			&corev1.Event{},
			//&source.Kind{Type: &corev1.Event{}},
			handler.EnqueueRequestsFromMapFunc(r.mapEventToRequest),
		).
		Complete(r)
}

// // SetupWithManager sets up the controller with the Manager.
// func (r *RedEnvoyReconciler) SetupWithManager(mgr ctrl.Manager) error {
// 	return ctrl.NewControllerManagedBy(mgr).
// 		For(&appsv1.RedEnvoy{}).
// 		Named("apps-redenvoy").
// 		Complete(r)
// }
