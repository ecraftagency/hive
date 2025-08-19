.PHONY: build clean agent server client web deploy-agent deploy-server deploy-web

# Build all components
build: agent server client web

# Build Agent
agent:
	go build -o bin/agent cmd/agent/main.go

# Build Server  
server:
	go build -o bin/server cmd/server/main.go

# Build Client
client:
	go build -o bin/client cmd/client/main.go

# Build Web Client
web:
	go build -o bin/web cmd/web/main.go

# Deploy Agent to remote host
deploy-agent:
	./scripts/deploy-agent.sh

# Deploy Server to remote host (no Nomad job control)
deploy-server:
	./scripts/deploy-server.sh

# Deploy Web client to remote host (no Nomad job control)
deploy-web:
	./scripts/deploy-web.sh

# Clean build artifacts
clean:
	rm -rf bin/

# Run Agent
run-agent: agent
	./bin/agent

# Run Server
run-server: server
	./bin/server

# Run Client
run-client: client
	./bin/client

# Run Web
run-web: web
	./bin/web
