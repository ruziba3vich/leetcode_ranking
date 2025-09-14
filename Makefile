MIGRATIONS_DIR=./db/migrations
MIGRATIONS_PATH=./db/migrations

DB_URL=postgres://leetcode_rankings_user:leetcode_rankings_pwd@89.117.58.248:5433/leetcode_rankings?sslmode=disable

# ===================================================================
# Create a new migration
# ===================================================================
create-migration:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $$name

.PHONY: migrate-up
migrate-up:
	docker run --rm \
		-v $(PWD)/db/migrations:/migrations \
		--network=host migrate/migrate \
		-path=/migrations -database "$(DB_URL)" up && \
	echo "✅ Database migrations applied successfully from $(MIGRATIONS_PATH)"


swag-gen:
	swag init -g cmd/main.go -o docs --parseDependency --parseInternal
