REPO = ..

all: run

# Run fwbuild-manager locally at the port 8000
run:
	$(MAKE) -C $(REPO)/docker/fwbuild-instance
	$(MAKE) -C $(REPO)/docker/fwbuild-manager
	docker-compose up
