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
	PodName   string `json:"podName"`
	Action    string `json:"action"`    // Expected to be "create" or "delete"
	CRName    string `json:"crName"`    // The name of your RedEnvoy custom resource
	Namespace string `json:"namespace"` // The namespace of your RedEnvoy custom resource
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
			Message: reqData.PodName, // Your operator uses the message as the Pod name
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

		log.Printf("Successfully created event %s to %s %s", createdEvent.Name, reqData.Action, reqData.PodName)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Event %s created successfully\n", createdEvent.Name)))
	})

	log.Println("Starting web server on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
