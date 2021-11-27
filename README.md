## jibu test
prerequisite:
1. a cluster with jibu rest server installed
2. a tenant with at least one cluster and one storage configured

steps to execute the test:
1. edit ./test/jibu/global.go as needed, mainly the `jibuAPIEndpoint` and `tenant` variables.
2. go test -v -timeout 4h ./test/jibu/... -args -ginkgo.v