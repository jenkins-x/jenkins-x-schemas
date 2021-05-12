module github.com/jenkins-x/jenkins-x-schemas

require (
	github.com/google/go-cmp v0.5.5 // indirect
	github.com/google/uuid v1.1.5 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/jenkins-x/go-scm v1.6.14
	github.com/jenkins-x/jx-api/v4 v4.0.33 // indirect
	github.com/jenkins-x/jx-helpers/v3 v3.0.104
	github.com/jenkins-x/jx-logging/v3 v3.0.3
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mitchellh/reflectwalk v1.0.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.1 // indirect
	github.com/stretchr/testify v1.6.1
	k8s.io/apiextensions-apiserver v0.21.0 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.20.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.3
	k8s.io/client-go => k8s.io/client-go v0.20.3
)

go 1.15
