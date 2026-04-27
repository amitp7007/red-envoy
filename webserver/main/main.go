package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// WebhookRequest defines the expected JSON body from Postman
type WebhookRequest struct {
	PodName   string            `json:"podName"`
	Action    string            `json:"action"`    // Expected to be "create" or "delete"
	CRName    string            `json:"crName"`    // The name of your RedEnvoy custom resource
	Namespace string            `json:"namespace"` // The namespace of your RedEnvoy custom resource
	Envs      map[string]string `json:"envs,omitempty"`
}

// EventPayload defines the structure of the message that will be sent in the event.
// It will be marshalled into a JSON string.
type EventPayload struct {
	PodName string            `json:"podName"`
	Envs    map[string]string `json:"envs,omitempty"`
}

// StatusResponse defines the JSON body for the status API
type StatusResponse struct {
	PodStatus     string `json:"podStatus"`
	ServiceStatus string `json:"serviceStatus"`
	IngressStatus string `json:"ingressStatus"`
}

func main() {
	var kubeconfig *string
	if home := os.Getenv("HOME"); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// Setup Kubernetes client
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Printf("Falling back to in-cluster config due to: %v", err)
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("Failed to load Kubernetes config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Setup HTTP handler
	http.HandleFunc("/trigger", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		var reqData WebhookRequest
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse JSON: %v", err), http.StatusBadRequest)
			return
		}

		// Map the action to the corresponding Event Reason your operator expects
		var reason string
		if reqData.Action == "create" {
			reason = "CreateNodredPod" // This must match TriggeredEventName in your CR Spec
		} else if reqData.Action == "delete" {
			reason = "DeleteNodredPod" // This must match TriggeredDeleteEventName in your CR Spec
		} else {
			http.Error(w, "Invalid action. Must be 'create' or 'delete'", http.StatusBadRequest)
			return
		}

		// The message will now be a JSON payload containing the pod name and envs
		payload := EventPayload{
			PodName: reqData.PodName,
			Envs:    reqData.Envs,
		}
		log.Printf("Received request to %s pod %s with envs: %v", reqData.Action, reqData.PodName, reqData.Envs)
		messageBytes, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to serialize event payload: %v", err), http.StatusInternalServerError)
			return
		}

		// Construct the Kubernetes Event
		event := &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "NoderedPodEvent-",
				Namespace:    reqData.Namespace,
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:       "RedEnvoy",
				Name:       reqData.CRName,
				Namespace:  reqData.Namespace,
				APIVersion: "apps.abstractprism.com/v1",
			},
			Reason:  reason,
			Message: string(messageBytes), // Your operator parses this JSON string
			Type:    corev1.EventTypeNormal,
			Source: corev1.EventSource{
				Component: "RedEnvoyWebhook",
			},
		}

		// Create the Event in the cluster
		createdEvent, err := clientset.CoreV1().Events(reqData.Namespace).Create(context.Background(), event, metav1.CreateOptions{})
		if err != nil {
			log.Printf("Error creating event: %v", err)
			http.Error(w, fmt.Sprintf("Failed to create Kubernetes event: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("Successfully created event %s for %s %s", createdEvent.Name, reqData.Action, reqData.PodName)

		// Return the expected resource information
		resp := map[string]interface{}{
			"message":   fmt.Sprintf("Event %s created successfully", createdEvent.Name),
			"eventName": createdEvent.Name,
			"expectedResources": map[string]string{
				"pod":     reqData.PodName,
				"service": reqData.PodName,
				"ingress": reqData.PodName,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	})

	// Setup the new Status API
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
			return
		}

		namespace := r.URL.Query().Get("namespace")
		podName := r.URL.Query().Get("podName")

		if namespace == "" || podName == "" {
			http.Error(w, "Missing 'namespace' or 'podName' query parameters", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		res := StatusResponse{
			PodStatus:     "NotFound",
			ServiceStatus: "NotFound",
			IngressStatus: "NotFound",
		}

		// Check Pod
		if pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{}); err == nil {
			res.PodStatus = string(pod.Status.Phase)
		}

		// Check Service
		if _, err := clientset.CoreV1().Services(namespace).Get(ctx, podName, metav1.GetOptions{}); err == nil {
			res.ServiceStatus = "Found"
		}

		// Check Ingress
		if _, err := clientset.NetworkingV1().Ingresses(namespace).Get(ctx, podName, metav1.GetOptions{}); err == nil {
			res.IngressStatus = "Found"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	})

	log.Println("Starting web server on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
