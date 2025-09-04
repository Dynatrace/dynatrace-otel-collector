.PHONY: install
install:
	@echo "-----------------------------------"
	@echo "Create backend receivers"
	@echo "-----------------------------------"
	kubectl apply -f collector-receiver1
	kubectl wait --for=condition=available deployment/collector-receiver1 --timeout=300s
	kubectl apply -f collector-receiver2
	kubectl wait --for=condition=available deployment/collector-receiver2 --timeout=300s
	kubectl apply -f collector-receiver3
	kubectl wait --for=condition=available deployment/collector-receiver3 --timeout=300s
	kubectl apply -f collector-receiver4
	kubectl wait --for=condition=available deployment/collector-receiver4 --timeout=300s

	@echo "-----------------------------------"
	@echo "Create load balancer"
	@echo "-----------------------------------"
	kubectl apply -f collector-loadbalancer
	kubectl wait --for=condition=available deployment/collector-loadbalancer --timeout=300s

scale-up-loadbalancer: install
	kubectl scale deployment collector-loadbalancer --replicas 4 --timeout=300s

scale-down-loadbalancer: install
	kubectl scale deployment collector-loadbalancer --replicas 2 --timeout=300s

delete-receivers:
	kubectl delete -f collector-receiver2 --ignore-not-found
	kubectl delete -f collector-receiver4 --ignore-not-found

deploy-receivers:
	kubectl apply -f collector-receiver2
	kubectl wait --for=condition=available deployment/collector-receiver2 --timeout=300s
	kubectl apply -f collector-receiver4
	kubectl wait --for=condition=available deployment/collector-receiver4 --timeout=300s

.PHONY: generate-load
generate-load:
	@echo "-----------------------------------"
	@echo "Create load generators"
	@echo "-----------------------------------"
# 	kubectl apply -f telemetrygen-logs
# 	kubectl wait --for=condition=available deployment/temeletrygen-logs-deployment --timeout=300s
# 	kubectl apply -f telemetrygen-metrics
# 	kubectl wait --for=condition=available deployment/temeletrygen-metrics-deployment --timeout=300s
	kubectl apply -f telemetrygen-traces
	kubectl wait --for=condition=available deployment/temeletrygen-traces-deployment --timeout=300s

.PHONY: stop-load
stop-load:
	@echo "-----------------------------------"
	@echo "Stop load generators"
	@echo "-----------------------------------"
# 	kubectl delete -f telemetrygen-metrics --ignore-not-found
# 	kubectl delete -f telemetrygen-logs --ignore-not-found
	kubectl delete -f telemetrygen-traces --ignore-not-found

.PHONY: uninstall
uninstall:
	kubectl delete -f collector-loadbalancer --ignore-not-found
	kubectl delete -f collector-receiver1 --ignore-not-found
	kubectl delete -f collector-receiver2 --ignore-not-found
	kubectl delete -f collector-receiver3 --ignore-not-found
	kubectl delete -f collector-receiver4 --ignore-not-found
