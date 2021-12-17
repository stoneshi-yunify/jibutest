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

```shell
go test -v -timeout 4h ./test/jibu/... -args -ginkgo.v -jibu-tenant=381577994897986984 -jibu-api-endpoint="http://192.168.0.15:31800" -jibu-backup-repeat-enabled=true -jibu-backup-method=snapshot -jibu-backup-frequency="*/1 * * * *" -jibu-backup-namespace=kubesphere-monitoring-system -jibu-restore-namespace=kubesphere-monitoring-system
```
