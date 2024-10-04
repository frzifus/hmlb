all:
	@echo "do nothing"

bootstrap-k8s-homelab:
	@kubectl apply -k clusters/homelab/bootstrap
