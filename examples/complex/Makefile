.PHONY: all check gen verify clean

ASSETS ?= assets

.DEFAULT_GOAL := all

-include .assets.mk

all: .assets.mk $(GENERATED_ASSET_FILES)
	@echo "assets up to date"

.assets.mk: assets.yaml
	$(ASSETS) gen --manifest assets.yaml > .assets.mk

check:
	$(ASSETS) check --manifest assets.yaml --strict

gen:
	$(ASSETS) gen --manifest assets.yaml > .assets.mk

$(GENERATED_ASSET_FILES): .assets.mk
	$(ASSETS) build --manifest assets.yaml --target $@

verify:
	$(ASSETS) verify --manifest assets.yaml

clean:
	rm -f .assets.mk assets.lock
	rm -rf out
