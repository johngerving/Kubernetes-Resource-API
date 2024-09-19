# Kubernetes Resource API

A RESTful API for querying available resources in a Kubernetes cluster.

## Getting started

Clone the repository with ```git clone https://gitlab.nrp-nautilus.io/humboldt/kubernetes-resource-api.git```.

Run tests with ```go test```.

Start the API server locally with ```go run . ./config_sa```. You must have a Kubernetes Service Account config file with the ClusterRole rolebinding named ```config_sa``` in the same directory. The service will then be available on ```localhost:8080```.

Build the Docker container with ```docker build -t <name of image> .``` (Note: the Docker image cannot be tested locally, as there is no Kubernetes config file mounted in the Docker container.)

To deploy the API on a Kubernetes cluster, use ```kubectl apply -f``` on each file in the ```deploy/``` directory. In order to run, the ```config-volume``` created by ```deploy/config-volume.yaml``` must contain a Kubernetes Service Account config file with the ClusterRole rolebinding.

## Endpoints

### /nodes

Returns a list of every node in the cluster. Each node contains information on the name of the node, its taints, its allocatable resources, resource capacity, and free resources. Each of these resource objects contain the number of CPUs as a float, the amount of memory in bytes, the number of GPUs as an integer, and the amount of ephemeral storage in bytes.

### Dockerfile

The Dockerfile contains two build stages: one builds the Go source code on a regular Go-based image, and the other has the resulting binary copied into it. This second image is what is actually built by Docker and results in a much lighter image.