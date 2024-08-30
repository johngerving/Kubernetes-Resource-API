package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"
)

type Resources struct {
	Cpu       resource.Quantity
	Memory    resource.Quantity
	Gpu       resource.Quantity
	Ephemeral resource.Quantity
}

type Node struct {
	Name        string
	Taints      []corev1.Taint
	Allocatable Resources
	Capacity    Resources
	Free        Resources
}

type NodeResources struct {
	Allocatable Resources
	Requests    map[corev1.ResourceName]resource.Quantity
	Limits      map[corev1.ResourceName]resource.Quantity
}

func main() {

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
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)

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

	nodes := make(map[string]*Node)

	err = getNodeInfo(clientset, nodes)

	if err != nil {
		panic(err)
	}

	err = getNodeFreeResources(clientset, nodes)

	if err != nil {
		panic(err)
	}

	for _, node := range nodes {
		bytes, _ := json.MarshalIndent(node, "\t", "\t")
		fmt.Println(string(bytes))
	}
}

// getNodeInfo modifies a map of Node instances, adding entries with the node name as a key.
// It gets the name of the node, its taints, capacity, and allocatable resources. These are added to the nodes map.
func getNodeInfo(client kubernetes.Interface, nodes map[string]*Node) error {
	// Get all nodes in the cluster
	nodeList, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})

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
	nonTerminatedPods, err := kubeClient.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{FieldSelector: "status.phase!=" + string(corev1.PodSucceeded) + ",status.phase!=" + string(corev1.PodFailed)})

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

// getNodeRequestsAndLimits modifies a NodeResources struct instance and sums the requests and limits of each resource for every pod in every node
func getNodeRequestsAndLimits(kubeClient kubernetes.Interface, nodeResources map[string]NodeResources) {
	// Get a list of every pod in the cluster that isn't terminated
	nonTerminatedPods, err := kubeClient.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{FieldSelector: "status.phase!=" + string(corev1.PodSucceeded) + ",status.phase!=" + string(corev1.PodFailed)})

	if err != nil {
		panic(err.Error())
	}

	for _, pod := range nonTerminatedPods.Items {
		// If nodeResources does not have an entry with the node name, we allocate a new NodeResources struct instance for it
		if _, ok := nodeResources[pod.Spec.NodeName]; !ok {
			nodeResources[pod.Spec.NodeName] = NodeResources{Requests: make(map[corev1.ResourceName]resource.Quantity), Limits: make(map[corev1.ResourceName]resource.Quantity)}
		}

		// Get the requests and limits for the pod
		podReqs, podLimits := resourcehelper.PodRequestsAndLimits(&pod)

		// Go through request names and values in the pod
		for podReqName, podReqValue := range podReqs {
			if value, ok := nodeResources[pod.Spec.NodeName].Requests[podReqName]; !ok {
				// If the request ResourceName doesn't exist, create it and copy the resource Quantity to it
				nodeResources[pod.Spec.NodeName].Requests[podReqName] = podReqValue.DeepCopy()
			} else {
				// Otherwise, add the resource Quantity to the existing value in nodeResources
				value.Add(podReqValue)
				nodeResources[pod.Spec.NodeName].Requests[podReqName] = value
			}
		}

		// Go through limit names and values in the pod
		for podLimitName, podLimitValue := range podLimits {
			if value, ok := nodeResources[pod.Spec.NodeName].Limits[podLimitName]; !ok {
				// If the limit ResourceName doesn't exist, create it and copy the resource Quantity to it
				nodeResources[pod.Spec.NodeName].Limits[podLimitName] = podLimitValue.DeepCopy()
			} else {
				// Otherwise, add the resource Quantity to the existing value in nodeResources
				value.Add(podLimitValue)
				nodeResources[pod.Spec.NodeName].Limits[podLimitName] = value
			}
		}
	}
}
