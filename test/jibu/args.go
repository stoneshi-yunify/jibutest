package jibu

import (
	"context"
	"flag"
	"time"
)

const (
	resourcePickRetryLimit    = 10
	jobRetention              = 240
	backupPlanReadyTimeout    = 5 * time.Minute
	backupJobFinishedTimeout  = 2 * time.Hour
	restorePlanReadyTimeout   = 5 * time.Minute
	restoreJobFinishedTimeout = 2 * time.Hour
	actionStartJob            = "StartJob"
)

// flags
var (
	argTenant = flag.String("jibu-tenant", "1", "tenant id")
	argJibuAPIEndpoint = flag.String("jibu-api-endpoint", "http://localhost:31800", "jibu api endpoint")
	argExcludeNamespaces = flag.String("jibu-exclude-namespaces", "kube-system,kube-public,kube-node-lease,qiming-backend,backup-saas-system", "exclude namespaces for backup and restore, separated by comma")
	argRestoreToSameNamespace = flag.Bool("jibu-restore-same-namespace", false, "restore uses same namespace as backup")
	argBackupWithPV = flag.Bool("jibu-backup-with-pv", true, "backup with pv")
	argBackupNamespace = flag.String("jibu-backup-namespace", "", "if set, backup specified namespace")
	argRestoreNamespace = flag.String("jibu-restore-namespace", "", "if set, restore to the specified namespace")
	argSkipBackup = flag.Bool("jibu-skip-backup", false, "if set, skip backup test")
	argSkipRestore = flag.Bool("jibu-skip-restore", false, "if set, skip restore test")
	argBackupCluster = flag.String("jibu-backup-cluster", "", "if set, use specified cluster for backup")
	argRestoreCluster = flag.String("jibu-restore-cluster", "", "if set, use specified cluster for restore")
	argStorage = flag.String("jibu-storage", "", "if set, use specified storage during test")
	argCleanUpOnEnd = flag.Bool("jibu-clean-up-on-end", true, "clean up after test, including delete backup/restore jobs, delete restored namespace etc.")
	argBackupPlanName = flag.String("jibu-backup-plan-name", "", "backup plan name, if not set, will use backup-{timestamp}")
	argBackupJobName = flag.String("jibu-backup-job-name", "", "backup job name, if not set, will use {backup-plan-name}-{random-string}")
	argRestorePlanName = flag.String("jibu-restore-plan-name", "", "restore plan name, if not set, will use restore-{timestamp}")
	argRestoreJobName = flag.String("jibu-restore-job-name", "", "restore job name, if not set, will use {restore-plan-name}-{random-string}")
)

var ctx = context.Background()
