.PHONY: local-setup
local-setup:
	@echo ""
	@echo "deleting kind cluster..."
	@$(MAKE) kind-delete-cluster
	@echo ""
	@echo "creating kind cluster..."
	@$(MAKE) kind-create-cluster
	@echo ""
	@echo "installing developer portal controller APIs..."
	@$(MAKE) install
	@echo ""
	@echo "installing Gateway API APIs..."
	@$(MAKE) gateway-api-install
	@echo ""
	@echo "installing kuadrant core APIs..."
	@$(MAKE) kuadrant-core-install
	@echo ""
	@echo "installing demo resources..."
	@$(MAKE) demo-install
	@echo ""
	@echo "cluster ready! kuadrant and demo resources installed."
	@echo ""
