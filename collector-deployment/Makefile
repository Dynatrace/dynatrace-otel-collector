.PHONY: install
install:
	@echo "-----------------------------------"
	@echo "Create backend receivers"
	@echo "-----------------------------------"
	kubectl apply -f collector-receiver1
	kubectl wait --for=condition=available deployment/collector-receiver1 --timeout=300s
	kubectl apply -f collector-receiver2
	kubectl wait --for=condition=available deployment/collector-receiver2 --timeout=300s

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
	kubectl apply -f telemetrygen
	kubectl wait --for=condition=available deployment/temeletrygen-metrics-deployment --timeout=300s
	kubectl apply -f telemetrygen2
	kubectl wait --for=condition=available deployment/temeletrygen-metrics-deployment2 --timeout=300s
	kubectl apply -f telemetrygen3
	kubectl wait --for=condition=available deployment/temeletrygen-metrics-deployment3 --timeout=300s
	kubectl apply -f telemetrygen4
	kubectl wait --for=condition=available deployment/temeletrygen-metrics-deployment4 --timeout=300s

.PHONY: uninstall
uninstall:
	kubectl delete -f telemetrygen
	kubectl delete -f telemetrygen2
	kubectl delete -f telemetrygen3
	kubectl delete -f telemetrygen4
	kubectl delete -f collector-loadbalancer
	kubectl delete -f collector-receiver1
	kubectl delete -f collector-receiver2
