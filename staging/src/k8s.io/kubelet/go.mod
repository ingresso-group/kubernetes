// This is a generated file. Do not edit directly.

module k8s.io/kubelet

go 1.16

require (
	github.com/gogo/protobuf v1.3.2
	golang.org/x/net v0.0.0-20211209124913-491a49abca63
	google.golang.org/genproto v0.0.0-20201110150050-8816d57aaa9a // indirect
	google.golang.org/grpc v1.27.1
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/component-base v0.0.0
)

replace (
	github.com/cespare/xxhash/v2 => github.com/cespare/xxhash/v2 v2.1.1
	github.com/google/go-cmp => github.com/google/go-cmp v0.5.5
	github.com/onsi/ginkgo => github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega => github.com/onsi/gomega v1.7.0
	k8s.io/api => ../api
	k8s.io/apimachinery => ../apimachinery
	k8s.io/client-go => ../client-go
	k8s.io/component-base => ../component-base
	k8s.io/kubelet => ../kubelet
)
