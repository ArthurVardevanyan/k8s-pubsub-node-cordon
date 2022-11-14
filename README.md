# K8S PubSub Node Cordon Script

OKD/GKE/K8S/K3S Cluster -> CLoud Logging -> Log Based Metric -> Alert Policy -> Pub/Sub -> Go Script On Deployment -> Cordon Nodes on List

## Build

```bash
export KO_DOCKER_REPO=

ko build --platform=linux/amd64 --bare --sbom none --tags 0.0.#1 # Quay Doesn't Support SBOM KO Yet
```

## Run

```bash
export PROJECT_ID=""
export SUB_ID=""
export CLUSTER='okd' #GKE #K3S
export IS_LOCAL=true #false

go run main.go
```
