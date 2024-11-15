all:
	@echo "do nothing"

bootstrap-k8s-homelab:
	@kubectl apply -k clusters/homelab/bootstrap
	@kubectl -n flux-system create secret generic sops-age --from-file=age.agekey=sops-key.txt
