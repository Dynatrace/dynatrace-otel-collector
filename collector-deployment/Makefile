.PHONY: install
install:
	@echo "-----------------------------------"
	@echo "Create backend receivers"
	@echo "-----------------------------------"
	kubectl apply -f collector-metrics-receiver
	kubectl apply -f collector-traces-receiver
	kubectl apply -f collector-logs-receiver
	kubectl wait --for=condition=available deployment/collector-metrics-receiver --timeout=300s
	kubectl wait --for=condition=available deployment/collector-traces-receiver --timeout=300s
	kubectl wait --for=condition=available deployment/collector-logs-receiver --timeout=300s

	@echo "-----------------------------------"
	@echo "Create load balancer"
	@echo "-----------------------------------"
	kubectl apply -f collector-loadbalancer
	kubectl wait --for=condition=available deployment/collector-loadbalancer --timeout=300s

scale-up-loadbalancer: install
	kubectl scale deployment collector-loadbalancer --replicas 2 --timeout=300s

scale-down-loadbalancer: install
	kubectl scale deployment collector-loadbalancer --replicas 1 --timeout=300s

.PHONY: generate-load
generate-load:
	@echo "-----------------------------------"
	@echo "Create load generators"
	@echo "-----------------------------------"
	kubectl apply -f metrics-gen-1
	kubectl wait --for=condition=available deployment/temeletrygen-metrics-deployment1 --timeout=300s
	kubectl apply -f metrics-gen-2
	kubectl wait --for=condition=available deployment/temeletrygen-metrics-deployment2 --timeout=300s
	kubectl apply -f traces-gen
	kubectl wait --for=condition=available deployment/temeletrygen-traces-deployment --timeout=300s
	kubectl apply -f logs-gen
	kubectl wait --for=condition=available deployment/temeletrygen-logs-deployment --timeout=300s

.PHONY: uninstall
uninstall:
	kubectl delete -f metrics-gen-1 --ignore-not-found
	kubectl delete -f metrics-gen-2 --ignore-not-found
	kubectl delete -f traces-gen --ignore-not-found
	kubectl delete -f logs-gen --ignore-not-found
	kubectl delete -f collector-loadbalancer --ignore-not-found
	kubectl delete -f collector-metrics-receiver --ignore-not-found
	kubectl delete -f collector-traces-receiver --ignore-not-found
	kubectl delete -f collector-logs-receiver --ignore-not-found
