REPO = ..

all: run

# Run fwbuild-manager locally at the port 8000
run:
	$(MAKE) -C $(REPO)/mos-tool_tools/docker/fwbuild-instance
	# $(MAKE) -C $(REPO)/mos-tool_tools/docker/fwbuild-manager
	docker-compose up
