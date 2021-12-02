## jibu test
prerequisite:
1. a cluster with jibu rest server installed
2. a tenant with at least one cluster and one storage configured

run below command to start the test:
```shell
go test -v -timeout 4h ./test/jibu/... -args -ginkgo.v \
-jibu-tenant=369641743475338021 \
-jibu-api-endpoint="http://103.33.66.159:33800"
```