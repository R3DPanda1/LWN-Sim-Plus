CLUSTER_NAME := lorawan
NAMESPACE := lorawan
IMAGE_NAME := lwn-simulator:latest
REPO_ROOT := $(shell dirname $(CURDIR))

.PHONY: cluster-create cluster-delete build deploy teardown seed status logs up down rebuild

cluster-create:
	k3d cluster create --config k3d-config.yaml

cluster-delete:
	k3d cluster delete $(CLUSTER_NAME)

build:
	docker build -t $(IMAGE_NAME) $(REPO_ROOT)
	k3d image import $(IMAGE_NAME) -c $(CLUSTER_NAME)

deploy:
	kubectl apply -f namespace.yaml
	kubectl apply -f configmaps/
	kubectl apply -f infrastructure/
	@echo "Waiting for infrastructure..."
	kubectl wait -n $(NAMESPACE) --for=condition=ready pod -l app=postgres --timeout=60s
	kubectl wait -n $(NAMESPACE) --for=condition=ready pod -l app=redis --timeout=60s
	kubectl wait -n $(NAMESPACE) --for=condition=ready pod -l app=mosquitto --timeout=60s
	kubectl apply -f applications/

teardown:
	-kubectl delete -f applications/ --ignore-not-found
	-kubectl delete -f infrastructure/ --ignore-not-found
	-kubectl delete -f configmaps/ --ignore-not-found
	-kubectl delete pvc -n $(NAMESPACE) --all
	-kubectl delete -f namespace.yaml --ignore-not-found

seed:
	@echo "Waiting for all pods to be ready..."
	kubectl wait -n $(NAMESPACE) --for=condition=ready pod --all --timeout=180s
	python3 seed.py

status:
	kubectl get all -n $(NAMESPACE)

logs:
	@echo "Usage: make logs APP=simulator"
	@echo "Available: simulator, chirpstack, chirpstack-gateway-bridge, chirpstack-rest-api, postgres, redis, mosquitto"
	@test -n "$(APP)" && kubectl logs -f -n $(NAMESPACE) deployment/$(APP) || true

up: cluster-create build deploy
	@echo "Cluster is up. Run 'make seed' to bootstrap ChirpStack and simulator."

down: teardown cluster-delete

rebuild: build
	kubectl rollout restart -n $(NAMESPACE) deployment/simulator
	kubectl rollout status -n $(NAMESPACE) deployment/simulator --timeout=60s
