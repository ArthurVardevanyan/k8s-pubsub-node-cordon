package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"cloud.google.com/go/pubsub"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value bool   `json:"value"`
}

func in_cluster_login() *kubernetes.Clientset {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return clientset
}

func out_cluster_login() *kubernetes.Clientset {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return clientset
}

// https://stackoverflow.com/questions/62766115/how-to-cordon-a-kubernetes-node-using-golang-client
func cordon_node(clientset *kubernetes.Clientset, node_name string) bool {
	payload := []patchStringValue{{
		Op:    "replace",
		Path:  "/spec/unschedulable",
		Value: true,
	}}

	payloadBytes, _ := json.Marshal(payload)
	output, err := clientset.CoreV1().Nodes().Patch(context.TODO(), node_name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
	if err != nil {
		//panic(err.Error())
		return false
	}

	if output.Spec.Unschedulable {
		fmt.Println(output.Name + ": Node Cordoned")
	}

	return true
}

func pullMsgs(projectID, subID, cluster string, clientset *kubernetes.Clientset) error {
	// projectID := "my-project-id"
	// subID := "my-sub"
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return fmt.Errorf("pubsub.NewClient: %v", err)
	}
	defer client.Close()

	sub := client.Subscription(subID)

	// Receive messages for 10 seconds, which simplifies testing.
	// Comment this out in production, since `Receive` should
	// be used as a long running operation.
	// ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	// defer cancel()

	err = sub.Receive(ctx, func(_ context.Context, msg *pubsub.Message) {

		//fmt.Printf("Got message: %q\n", string(msg.Data))
		jsonString := msg.Data
		var jsonMap map[string]interface{}
		json.Unmarshal([]byte(jsonString), &jsonMap)
		var node_name = jsonMap["incident"].(map[string]interface{})["metric"].(map[string]interface{})["labels"].(map[string]interface{})["Node"].(string)

		fmt.Printf("Going to Cordon: %q\n", string(node_name))

		if strings.Contains(node_name, cluster) {
			if cordon_node(clientset, node_name) {
				msg.Ack()
			} else {
				fmt.Printf("Error, Skipping: %q\n", string(node_name))
			}
		} else {
			fmt.Printf("Awk & Ignoring: %q\n", string(node_name))
			msg.Ack()
		}
	})
	if err != nil {
		return fmt.Errorf("sub.Receive: %v", err)
	}

	return nil
}

// https://codereview.stackexchange.com/questions/108563/reading-environment-variables-of-various-types
func getStrEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic("Is the Variable Defined?")
	}
	return val
}

// func getIntEnv(key string) int {
// 	val := getStrEnv(key)
// 	ret, err := strconv.Atoi(val)
// 	if err != nil {
// 		panic("Is the Variable Defined?")
// 	}
// 	return ret
// }

func getBoolEnv(key string) bool {
	val := getStrEnv(key)
	ret, err := strconv.ParseBool(val)
	if err != nil {
		panic("Is the Variable Defined?")
	}
	return ret
}

func main() {
	fmt.Println("Application Booting")

	var clientset *kubernetes.Clientset

	var projectID = getStrEnv("PROJECT_ID")
	var subID = getStrEnv("SUB_ID")

	var cluster = getStrEnv("CLUSTER")

	fmt.Println("Logging Into Cluster")
	if getBoolEnv("IS_LOCAL") {
		clientset = out_cluster_login()
	} else {
		clientset = in_cluster_login()
	}
	fmt.Println("Pulling Messages")
	pullMsgs(projectID, subID, cluster, clientset)
}
