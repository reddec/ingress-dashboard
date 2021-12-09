local:
	rm -rf dist
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/ingress-dashboard ./cmd/ingress-dashboard
	cp Dockerfile.release dist/Dockerfile
	cd dist && docker build -t ingress-dashboard .