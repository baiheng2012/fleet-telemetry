.PHONY: up down test gen check dbt features quality

up:
	docker compose up -d
down:
	docker compose down
test:
	go vet ./... && go test ./... && pytest python/tests -q
gen:
	python -m fleet_tools.contract_codegen && python -m fleet_tools.dashboard_gen
check:
	python -m fleet_tools.contract_codegen --check
dbt:
	cd warehouse/dbt && DBT_PROFILES_DIR=$$PWD dbt build
features:
	python -m fleet_tools.features.build
quality:
	python -m fleet_tools.quality.runner -v
