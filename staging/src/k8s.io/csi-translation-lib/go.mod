// This is a generated file. Do not edit directly.

module k8s.io/csi-translation-lib

go 1.16

require (
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/klog/v2 v2.9.0
)

replace (
	github.com/google/go-cmp => github.com/google/go-cmp v0.5.5
	github.com/onsi/ginkgo => github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega => github.com/onsi/gomega v1.7.0
	k8s.io/api => ../api
	k8s.io/apimachinery => ../apimachinery
	k8s.io/csi-translation-lib => ../csi-translation-lib
)
