package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"
)

// Define resources struct containing the resource types we want to return
type Resources struct {
	Cpu       resource.Quantity
	Memory    resource.Quantity
	Gpu       resource.Quantity
	Ephemeral resource.Quantity
}

// Define node struct for storing resources and other node information
type Node struct {
	Name        string
	Taints      []corev1.Taint
	Allocatable Resources
	Capacity    Resources
	Free        Resources
}

type ResourcesJson struct {
	Cpu       float64 `json:"cpu"`
	Memory    int64   `json:"memory"`
	Gpu       int64   `json:"gpu"`
	Ephemeral int64   `json:"ephemeral"`
}

type NodeJson struct {
	Name        string         `json:"name"`
	Taints      []corev1.Taint `json:"taints"`
	Allocatable ResourcesJson  `json:"allocatable"`
	Capacity    ResourcesJson  `json:"capacity"`
	Free        ResourcesJson  `json:"free"`
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Println("error loading .env file")
	}

	var config *rest.Config

	// Get arguments after program name
	args := os.Args[1:]

	// Exit with error if kubeconfig path not provided
	if len(args) == 0 {
		fmt.Println("error: expected kubeconfig path")
		os.Exit(1)
	}

	// Get pointer to first argument after program name
	kubeconfig := &args[0]

	// Create a config from the kubeconfig file
	config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Create a Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Set to release mode depending on environment variable
	ginEnv := os.Getenv("GIN_MODE")
	if ginEnv == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	router.GET("/nodes", getNodesHandler(clientset))

	// Get port to run API on
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router.Run(":" + port)
}

// getNodesHandler returns a HandlerFunc to return a list of nodes given a Kubernetes clientset.
func getNodesHandler(client kubernetes.Interface) gin.HandlerFunc {
	// Define a handler function to return
	handler := func(c *gin.Context) {
		// Create a map of string to Node struct instances
		nodes := make(map[string]*Node)

		// Get the node capacity, allocatable resources, name, and taints
		err := getNodeInfo(client, nodes)

		if err != nil {
			fmt.Println(err)
			c.JSON(http.StatusInternalServerError, "error retrieving node information")
			return
		}

		// Get the available resources of the nodes
		err = getNodeFreeResources(client, nodes)

		if err != nil {
			fmt.Println(err)
			c.JSON(http.StatusInternalServerError, "error retrieving available node resources")
			return
		}

		// Create a slice to return the nodes instead of a map
		nodeSlice := make([]NodeJson, 0, len(nodes))

		for _, value := range nodes {
			nodeSlice = append(nodeSlice, getNodeStructured(value))
		}

		c.IndentedJSON(http.StatusOK, nodeSlice)
	}

	return gin.HandlerFunc(handler)
}

// getNodeStructured takes a pointer to a Node struct instance and returns a NodeJson struct instance
// with the fields properly converted
func getNodeStructured(node *Node) NodeJson {
	var nodeJson NodeJson

	// Copy name field
	nodeJson.Name = node.Name

	// If the node has no taints, add an empty slice - otherwise, copy the taints from the Node struct instance
	if node.Taints == nil {
		nodeJson.Taints = make([]corev1.Taint, 0)
	} else {
		nodeJson.Taints = node.Taints
	}

	// Copy the resource capacity fields and convert to numbers
	nodeJson.Capacity = ResourcesJson{
		Cpu:       node.Capacity.Cpu.AsApproximateFloat64(),
		Memory:    node.Capacity.Memory.Value(),
		Gpu:       node.Capacity.Gpu.Value(),
		Ephemeral: node.Capacity.Ephemeral.Value(),
	}

	// Copy the resource allocatable fields and convert to numbers
	nodeJson.Allocatable = ResourcesJson{
		Cpu:       node.Allocatable.Cpu.AsApproximateFloat64(),
		Memory:    node.Allocatable.Memory.Value(),
		Gpu:       node.Allocatable.Gpu.Value(),
		Ephemeral: node.Allocatable.Ephemeral.Value(),
	}

	// Copy the free resource fields and convert to numbers
	nodeJson.Free = ResourcesJson{
		Cpu:       node.Free.Cpu.AsApproximateFloat64(),
		Memory:    node.Free.Memory.Value(),
		Gpu:       node.Free.Gpu.Value(),
		Ephemeral: node.Free.Ephemeral.Value(),
	}

	return nodeJson
}

// getNodeInfo modifies a map of Node instances, adding entries with the node name as a key.
// It gets the name of the node, its taints, capacity, and allocatable resources. These are added to the nodes map.
func getNodeInfo(client kubernetes.Interface, nodes map[string]*Node) error {
	// Get all nodes in the cluster
	nodeList, err := client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})

	if err != nil {
		return err
	}

	// Loop through the nodes
	for _, node := range nodeList.Items {
		// Get the GPU capacity of the node - default 0
		gpuCapacity := node.Status.Capacity["nvidia.com/gpu"]

		// Loop through the fields of the node capacity
		for key, value := range node.Status.Capacity {
			// If the node is a GPU node, set the gpuCapacity to its GPU count
			if strings.HasPrefix(key.String(), "nvidia.com") && !value.IsZero() {
				gpuCapacity = value
			}
		}

		// Create a new Node with the correct resources
		newNode := Node{
			Name:   node.Name,
			Taints: node.Spec.Taints,
			Capacity: Resources{
				Cpu:       node.Status.Capacity.Cpu().DeepCopy(),
				Memory:    node.Status.Capacity.Memory().DeepCopy(),
				Gpu:       gpuCapacity,
				Ephemeral: node.Status.Capacity.StorageEphemeral().DeepCopy(),
			},
			Allocatable: Resources{
				Cpu:       node.Status.Allocatable.Cpu().DeepCopy(),
				Memory:    node.Status.Allocatable.Memory().DeepCopy(),
				Gpu:       gpuCapacity,
				Ephemeral: node.Status.Allocatable.StorageEphemeral().DeepCopy(),
			},
		}

		nodes[node.Name] = &newNode
	}

	return nil
}

// getNodeFreeResources modifies a map of Node instances and sums the requests
// of each resource for every pod in every node, subtracting them from the
// Allocatable resourcs.
func getNodeFreeResources(kubeClient kubernetes.Interface, nodes map[string]*Node) error {
	// Get a list of every pod in the cluster that isn't terminated
	nonTerminatedPods, err := kubeClient.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{FieldSelector: "status.phase!=" + string(corev1.PodSucceeded) + ",status.phase!=" + string(corev1.PodFailed)})

	if err != nil {
		return err
	}

	// For each node, copy the allocatable resources into the free resources to be subtracted from
	for _, node := range nodes {
		node.Free = Resources{
			Cpu:       node.Allocatable.Cpu.DeepCopy(),
			Memory:    node.Allocatable.Memory.DeepCopy(),
			Gpu:       node.Allocatable.Gpu.DeepCopy(),
			Ephemeral: node.Allocatable.Ephemeral.DeepCopy(),
		}
	}

	for _, pod := range nonTerminatedPods.Items {
		// Only get pod requests if the nodes map has an entry for the node
		if _, ok := nodes[pod.Spec.NodeName]; !ok {
			continue
		}

		// Get the requests and limits for the pod
		podReqs, _ := resourcehelper.PodRequestsAndLimits(&pod)

		// Get the relevant resource requests from the pod
		cpuReq := podReqs[corev1.ResourceCPU]
		memReq := podReqs[corev1.ResourceMemory]

		// Get the GPU capacity of the node - default 0
		gpuReq := podReqs["nvidia.com/gpu"]

		// Loop through the fields of the podReqs
		for key, value := range podReqs {
			// If the node is a GPU node, set the gpuCapacity to its GPU count
			if strings.HasPrefix(key.String(), "nvidia.com") && !value.IsZero() {
				gpuReq = value
			}
		}

		ephemeralReq := podReqs[corev1.ResourceEphemeralStorage]

		nodes[pod.Spec.NodeName].Free.Cpu.Sub(cpuReq)
		nodes[pod.Spec.NodeName].Free.Memory.Sub(memReq)
		nodes[pod.Spec.NodeName].Free.Gpu.Sub(gpuReq)
		nodes[pod.Spec.NodeName].Free.Ephemeral.Sub(ephemeralReq)
	}

	return nil
}
