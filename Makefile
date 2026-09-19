.DEFAULT_GOAL := help

GO ?= go
ROM ?= .*

.PHONY: help test test-all test-rom

help:
	@echo 'make test                       Run quick tests (skip test ROMs)'
	@echo 'make test-all                   Run all tests, including test ROMs'
	@echo 'make test-rom                   Run only test ROM regressions'
	@echo 'make test-rom ROM=instr_timing   Run one test ROM regression'

test:
	$(GO) test -short ./...

test-all:
	$(GO) test -count=1 -timeout=10m ./...

test-rom:
	$(GO) test -count=1 -timeout=10m -run '^TestROM$$/^$(ROM)$$' .
