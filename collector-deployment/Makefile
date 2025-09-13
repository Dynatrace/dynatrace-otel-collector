.PHONY: install
install:
	@echo "-----------------------------------"
	@echo "Create backend receivers"
	@echo "-----------------------------------"
	kubectl apply -f collector-receiver1
	kubectl wait --for=condition=available deployment/collector-receiver1 --timeout=300s

	@echo "-----------------------------------"
	@echo "Create load balancer"
	@echo "-----------------------------------"
	kubectl apply -f collector-loadbalancer
	kubectl wait --for=condition=available deployment/collector-loadbalancer --timeout=300s

scale-up-loadbalancer: install
	kubectl scale deployment collector-loadbalancer --replicas 4 --timeout=300s

scale-down-loadbalancer: install
	kubectl scale deployment collector-loadbalancer --replicas 2 --timeout=300s

scale-up-receiver:
	kubectl scale deployment collector-receiver1 --replicas 4 --timeout=300s

scale-down-receiver:
	kubectl scale deployment collector-receiver1 --replicas 2 --timeout=300s

.PHONY: generate-load
generate-load:
	@echo "-----------------------------------"
	@echo "Create load generators"
	@echo "-----------------------------------"
	kubectl apply -f telemetrygen-traces
	kubectl wait --for=condition=available deployment/temeletrygen-traces-deployment --timeout=300s

.PHONY: stop-load
stop-load:
	@echo "-----------------------------------"
	@echo "Stop load generators"
	@echo "-----------------------------------"
	kubectl delete -f telemetrygen-traces --ignore-not-found

.PHONY: uninstall
uninstall:
	kubectl delete -f collector-loadbalancer --ignore-not-found
	kubectl delete -f collector-receiver1 --ignore-not-found
	kubectl delete -f telemetrygen-traces --ignore-not-found

