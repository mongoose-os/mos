REPO = ..

all: run

# Run fwbuild-manager locally at the port 8000
run:
	$(MAKE) -C $(REPO)/tools/docker/fwbuild-instance
	# $(MAKE) -C $(REPO)/tools/docker/fwbuild-manager
	docker-compose up
