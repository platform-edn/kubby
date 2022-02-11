module github.com/platform-edn/kubby

go 1.13

require (
	github.com/docker/docker v20.10.12+incompatible
	github.com/docker/go-connections v0.4.0
	helm.sh/helm/v3 v3.8.0
	k8s.io/api v0.23.3
	k8s.io/apimachinery v0.23.3
	k8s.io/client-go v0.23.3
	sigs.k8s.io/kind v0.11.1
)

require github.com/moby/sys/mount v0.3.0 // indirect
