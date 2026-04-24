demo-install:
	@echo "installing demo resources..."
	@kubectl apply -f utils/demo/gamestore.yaml
	@echo ""
	@echo "demo resources installed!"
