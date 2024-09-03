package main

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestGetNodeInfo calls getNodeInfo on a map[string]*Nodes, checking that the resources in the resulting map
// match the mock nodes' resource values.
func TestGetNodeInfo(t *testing.T) {
	// Create a fake Kubernetes client
	kubeClient := fake.NewClientset()

	// Create two fake nodes, node-1 and node-2
	newNodes := []v1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
			Status: v1.NodeStatus{
				Capacity: v1.ResourceList{
					v1.ResourceCPU:              *resource.NewMilliQuantity(24000, resource.DecimalSI),
					v1.ResourceMemory:           *resource.NewMilliQuantity(5000, resource.DecimalSI),
					v1.ResourceEphemeralStorage: *resource.NewMilliQuantity(32000, resource.DecimalSI),
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:              *resource.NewMilliQuantity(20000, resource.DecimalSI),
					v1.ResourceMemory:           *resource.NewMilliQuantity(4000, resource.DecimalSI),
					v1.ResourceEphemeralStorage: *resource.NewMilliQuantity(28000, resource.DecimalSI),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
			},
			Status: v1.NodeStatus{
				Capacity: v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(16000, resource.DecimalSI),
					v1.ResourceMemory: *resource.NewMilliQuantity(5000, resource.DecimalSI),
					"nvidia.com/gpu":  *resource.NewQuantity(2, resource.DecimalSI),
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(12000, resource.DecimalSI),
					v1.ResourceMemory: *resource.NewMilliQuantity(4000, resource.DecimalSI),
					"nvidia.com/gpu":  *resource.NewQuantity(2, resource.DecimalSI),
				},
			},
		},
	}

	// Create the nodes with the fake client
	for _, node := range newNodes {
		kubeClient.CoreV1().Nodes().Create(context.TODO(), &node, metav1.CreateOptions{})
	}

	// Create a map of strings to Node struct instances
	nodes := make(map[string]*Node)

	getNodeInfo(kubeClient, nodes)

	// Loop through the nodes added to the cluster
	for _, node := range newNodes {
		// Check that each node in the cluster exists in the Node resources map
		if _, ok := nodes[node.Name]; !ok {
			t.Fatalf(`map[string]*Node does not contain key %v`, node.Name)
		}

		switch {
		// Check the capacity of each resource to make sure it matches the nodes' values
		case !nodes[node.Name].Capacity.Cpu.Equal(node.Status.Capacity.Cpu().DeepCopy()):
			t.Fatalf(`nodes[%v].Capacity.Cpu = %v, want match for %v`, node.Name, &nodes[node.Name].Capacity.Cpu, node.Status.Capacity.Cpu())
		case !nodes[node.Name].Capacity.Memory.Equal(node.Status.Capacity.Memory().DeepCopy()):
			t.Fatalf(`nodes[%v].Capacity.Memory = %v, want match for %v`, node.Name, &nodes[node.Name].Capacity.Memory, node.Status.Capacity.Memory())
		case !nodes[node.Name].Capacity.Ephemeral.Equal(node.Status.Capacity.StorageEphemeral().DeepCopy()):
			t.Fatalf(`nodes[%v].Capacity.Ephemeral = %v, want match for %v`, node.Name, &nodes[node.Name].Capacity.Ephemeral, node.Status.Capacity.StorageEphemeral())

			// Check the allocatable resources to make sure they match the nodes' values
		case !nodes[node.Name].Allocatable.Cpu.Equal(node.Status.Allocatable.Cpu().DeepCopy()):
			t.Fatalf(`nodes[%v].Allocatable.Cpu = %v, want match for %v`, node.Name, &nodes[node.Name].Allocatable.Cpu, node.Status.Allocatable.Cpu())
		case !nodes[node.Name].Allocatable.Memory.Equal(node.Status.Allocatable.Memory().DeepCopy()):
			t.Fatalf(`nodes[%v].Allocatable.Memory = %v, want match for %v`, node.Name, &nodes[node.Name].Allocatable.Memory, node.Status.Allocatable.Memory())
		case !nodes[node.Name].Allocatable.Ephemeral.Equal(node.Status.Allocatable.StorageEphemeral().DeepCopy()):
			t.Fatalf(`nodes[%v].Allocatable.Ephemeral = %v, want match for %v`, node.Name, &nodes[node.Name].Allocatable.Ephemeral, node.Status.Allocatable.StorageEphemeral())
		}

		// Check that the GPU capacity in the Resources struct instance matches the node's GPU capacity
		if gpuCapacity, ok := node.Status.Capacity["nvidia.com/gpu"]; ok {
			if !gpuCapacity.Equal(nodes[node.Name].Capacity.Gpu.DeepCopy()) {
				t.Fatalf(`nodes[%v].Capacity.Gpu = %v, want match for %v`, node.Name, nodes[node.Name].Capacity.Gpu.Value(), gpuCapacity.Value())
			}
		}

		// Check that the GPU allocatable in the Resources struct instance matches the node's GPU allocatable
		if gpuAllocatable, ok := node.Status.Allocatable["nvidia.com/gpu"]; ok {
			if !gpuAllocatable.Equal(nodes[node.Name].Allocatable.Gpu.DeepCopy()) {
				t.Fatalf(`nodes[%v].Allocatable.Gpu = %v, want match for %v`, node.Name, nodes[node.Name].Allocatable.Gpu.Value(), gpuAllocatable.Value())
			}
		}
	}
}

// TestGetNodeFreeResources calls getNodeFreeResources on a map[string]*Nodes, checking that the free resources in
// the resulting map match the mock nodes' values.
func TestGetNodeFreeResources(t *testing.T) {
	// Create a fake Kubernetes client
	kubeClient := fake.NewClientset()

	// Create two fake nodes, node-1 and node-2
	newNodes := []v1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
			Status: v1.NodeStatus{
				Capacity: v1.ResourceList{
					v1.ResourceCPU:              *resource.NewMilliQuantity(24000, resource.DecimalSI),
					v1.ResourceMemory:           *resource.NewMilliQuantity(5000, resource.DecimalSI),
					v1.ResourceEphemeralStorage: *resource.NewMilliQuantity(32000, resource.DecimalSI),
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:              *resource.NewMilliQuantity(20000, resource.DecimalSI),
					v1.ResourceMemory:           *resource.NewMilliQuantity(4000, resource.DecimalSI),
					v1.ResourceEphemeralStorage: *resource.NewMilliQuantity(28000, resource.DecimalSI),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
			},
			Status: v1.NodeStatus{
				Capacity: v1.ResourceList{
					v1.ResourceCPU:              *resource.NewMilliQuantity(16000, resource.DecimalSI),
					v1.ResourceMemory:           *resource.NewMilliQuantity(16000, resource.DecimalSI),
					"nvidia.com/gpu":            *resource.NewQuantity(2, resource.DecimalSI),
					v1.ResourceEphemeralStorage: *resource.NewMilliQuantity(32000, resource.DecimalSI),
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:              *resource.NewMilliQuantity(12000, resource.DecimalSI),
					v1.ResourceMemory:           *resource.NewMilliQuantity(12000, resource.DecimalSI),
					"nvidia.com/gpu":            *resource.NewQuantity(2, resource.DecimalSI),
					v1.ResourceEphemeralStorage: *resource.NewMilliQuantity(28000, resource.DecimalSI),
				},
			},
		},
	}

	// Define two pods to be added to the cluster
	pod1 := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-1",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "ubuntu",
					Image: "ubuntu",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:              *resource.NewQuantity(4, resource.DecimalSI),
							v1.ResourceMemory:           *resource.NewQuantity(2, resource.DecimalSI),
							v1.ResourceEphemeralStorage: *resource.NewMilliQuantity(4000, resource.DecimalSI),
							"nvidia.com/gpu":            *resource.NewQuantity(1, resource.DecimalSI),
						},
					},
				},
			},
			NodeName: "node-2",
		},
	}
	pod2 := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-2",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "ubuntu",
					Image: "ubuntu",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:              *resource.NewQuantity(3, resource.DecimalSI),
							v1.ResourceMemory:           *resource.NewQuantity(5, resource.DecimalSI),
							v1.ResourceEphemeralStorage: *resource.NewMilliQuantity(8500, resource.DecimalSI),
						},
					},
				},
			},
			NodeName: "node-2",
		},
	}

	// Create the nodes with the fake client
	for _, node := range newNodes {
		kubeClient.CoreV1().Nodes().Create(context.TODO(), &node, metav1.CreateOptions{})
	}

	// Create the two pods on the cluster
	kubeClient.CoreV1().Pods("default").Create(context.TODO(), pod1, metav1.CreateOptions{})
	kubeClient.CoreV1().Pods("default").Create(context.TODO(), pod2, metav1.CreateOptions{})

	// Create a map[string]*Node to store the resources and requests
	nodes := make(map[string]*Node)
	// Get the capacity and allocatable for each node
	getNodeInfo(kubeClient, nodes)
	// Get the pod requests and subtract from the allocatable to get the free resources
	getNodeFreeResources(kubeClient, nodes)

	switch {
	// Test free resources for node-1 - should be equal to allocatable resources since no pods are on the node
	case !nodes["node-1"].Free.Cpu.Equal(*resource.NewQuantity(20, resource.DecimalSI)):
		t.Fatalf(`nodes[%v].Free.Cpu = %v, want match for %v`, "node-1", &nodes["node-1"].Free.Cpu, resource.NewQuantity(20, resource.DecimalSI).Value())
	case !nodes["node-1"].Free.Memory.Equal(*resource.NewMilliQuantity(4000, resource.DecimalSI)):
		t.Fatalf(`nodes[%v].Free.Memory = %v, want match for %v`, "node-1", &nodes["node-1"].Free.Memory, resource.NewMilliQuantity(4000, resource.DecimalSI).Value())
	case !nodes["node-1"].Free.Ephemeral.Equal(*resource.NewMilliQuantity(28000, resource.DecimalSI)):
		t.Fatalf(`nodes[%v].Free.Ephemeral = %v, want match for %v`, "node-1", &nodes["node-1"].Free.Ephemeral, resource.NewMilliQuantity(28000, resource.DecimalSI).Value())

	// Test free resources for node-2
	case !nodes["node-2"].Free.Cpu.Equal(*resource.NewQuantity(5, resource.DecimalSI)):
		t.Fatalf(`nodes[%v].Free.Cpu = %v, want match for %v`, "node-2", &nodes["node-2"].Free.Cpu, resource.NewQuantity(5, resource.DecimalSI).Value())
	case !nodes["node-2"].Free.Memory.Equal(*resource.NewMilliQuantity(5000, resource.DecimalSI)):
		t.Fatalf(`nodes[%v].Free.Memory = %v, want match for %v`, "node-2", &nodes["node-2"].Free.Memory, resource.NewMilliQuantity(5000, resource.DecimalSI).Value())
	case !nodes["node-2"].Free.Ephemeral.Equal(*resource.NewMilliQuantity(15500, resource.DecimalSI)):
		t.Fatalf(`nodes[%v].Free.Ephemeral = %v, want match for %v`, "node-2", &nodes["node-2"].Free.Ephemeral, resource.NewMilliQuantity(15500, resource.DecimalSI).Value())
	case !nodes["node-2"].Free.Gpu.Equal(*resource.NewQuantity(1, resource.DecimalSI)):
		t.Fatalf(`nodes[%v].Free.Gpu = %v, want match for %v`, "node-2", &nodes["node-2"].Free.Gpu, resource.NewQuantity(1, resource.DecimalSI).Value())
	}
}
