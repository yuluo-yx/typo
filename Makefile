_run:
	@$(MAKE) --warn-undefined-variables \
		-f tools/make/common.mk \
		-f tools/make/tools.mk \
		-f tools/make/golang.mk \
		-f tools/make/e2e.mk \
		$(MAKECMDGOALS)

.PHONY: _run

$(if $(MAKECMDGOALS),$(MAKECMDGOALS): %: _run)
