package jibu

import (
	"flag"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/antihax/optional"
	"github.com/davecgh/go-spew/spew"
	"github.com/robfig/cron/v3"

	"github.com/elliotchance/pie/pie"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	swagger "github.com/jibutech/backup-saas-client"
	"github.com/stoneshi-yunify/jibutest/pkg/utils/random"
	"k8s.io/apimachinery/pkg/util/wait"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBackupAndRestore(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	RegisterFailHandler(Fail)
	RunSpecs(t, "backup and restore")
}

var _ = Describe("use jibu api", func() {
	flag.Parse()
	flag.VisitAll(func(f *flag.Flag) {
		MyBy(fmt.Sprintf("%s = %v", f.Name, f.Value))
	})
	if err := validateFlags(); err != nil {
		panic(err.Error())
	}

	tenant := *argTenant
	jibuAPIEndpoint := *argJibuAPIEndpoint
	excludeNamespaces := pie.Strings(strings.Split(*argExcludeNamespaces, ","))
	restoreToSameNamespace := *argRestoreToSameNamespace
	backupRepeatEnabled := *argBackupRepeatEnabled
	backupRepeatCheckNum := *argBackupRepeatCheckNum
	backupFrequency := *argBackupFrequency
	backupWithPV := *argBackupWithPV
	backupCopyMethod := *argBackupCopyMethod
	backupNamespace := *argBackupNamespace
	restoreNamespace := *argRestoreNamespace
	skipBackup := *argSkipBackup
	skipRestore := *argSkipRestore
	backupCluster := *argBackupCluster
	restoreCluster := *argRestoreCluster
	backupStorage := *argStorage
	cleanUpOnEnd := *argCleanUpOnEnd
	backupPlanName := *argBackupPlanName
	backupJobName := *argBackupJobName
	restorePlanName := *argRestorePlanName
	restoreJobName := *argRestoreJobName

	var timestamp = time.Now().Format("20060102150405")
	if backupPlanName == "" {
		backupPlanName = strings.ToLower(fmt.Sprintf("backup-%v", timestamp))
	}
	if backupJobName == "" {
		backupJobName = strings.ToLower(fmt.Sprintf("%s-%s", backupPlanName, random.GetRandString(5)))
	}
	if restorePlanName == "" {
		restorePlanName = strings.ToLower(fmt.Sprintf("restore-%v", timestamp))
	}
	if restoreJobName == "" {
		restoreJobName = strings.ToLower(fmt.Sprintf("%s-%s", restorePlanName, random.GetRandString(5)))
	}

	jibuConf := swagger.NewConfiguration()
	jibuConf.BasePath = jibuAPIEndpoint
	jibuClient := swagger.NewAPIClient(jibuConf)

	Context("create a backup job and a restore job", func() {
		BeforeEach(func() {
			MyBy("clean up at the beginning")
			_, _, _ = jibuClient.BackupPlanTagApi.DeleteBackupPlan(ctx, tenant, backupPlanName)
			_ = deleteJobsOfBackupPlan(jibuClient, tenant, backupPlanName)
			_, _, _ = jibuClient.RestorePlanTagApi.DeleteRestorePlan(ctx, tenant, restorePlanName)
			_, _, _ = jibuClient.RestoreJobTagApi.DeleteRestoreJob(ctx, tenant, restoreJobName)
		})

		AfterEach(func() {
			if cleanUpOnEnd {
				MyBy("clean up at the end")
				// we only clean up the backup plan when it's repeated
				// because on-demand plan won't generate new jobs after the test finishes
				if backupRepeatEnabled {
					backupPlan, _, err := jibuClient.BackupPlanTagApi.GetBackupPlan(ctx, tenant, backupPlanName)
					if err == nil {
						MyBy(spew.Sdump("backup plan", backupPlan))
					}
					_, _, _ = jibuClient.BackupPlanTagApi.DeleteBackupPlan(ctx, tenant, backupPlanName)
				}
				_ = deleteJobsOfBackupPlan(jibuClient, tenant, backupPlanName)
				restoreJob, _, err := jibuClient.RestoreJobTagApi.GetRestoreJob(ctx, tenant, restoreJobName)
				if err == nil {
					MyBy(spew.Sdump("restorejob", restoreJob))
				}
				// _, _, _ = jibuClient.RestorePlanTagApi.DeleteRestorePlan(ctx, tenant, restorePlanName)
				_, _, _ = jibuClient.RestoreJobTagApi.DeleteRestoreJob(ctx, tenant, restoreJobName)
				if backupNamespace != restoreNamespace || backupCluster != restoreCluster {
					if len(backupCluster) != 0 && len(restoreNamespace) != 0 {
						// k8sClient := getK8sClientFromCluster(jibuClient, restoreClusterID)
						// deleteNamespace(k8sClient, restoredNamespace)
					}
				}
			}
		})

		It("should succeed", func() {
			var err error
			var meta swagger.V1ObjectMeta

			if !skipBackup {
				MyBy("pick a cluster for backup")
				cluster := pickOneCluster(jibuClient, tenant, backupCluster)
				backupCluster = cluster.Metadata.Name
				MyBy(fmt.Sprintf("cluster is picked, id=%s, display-name=%s", backupCluster, cluster.Spec.DisplayName))

				MyBy("pick a namespace")
				ns := pickOneNamespace(jibuClient, tenant, backupCluster, backupNamespace, excludeNamespaces)
				backupNamespace = ns.Metadata.Name
				MyBy(fmt.Sprintf("namespace %s is picked", backupNamespace))

				MyBy("pick a storage")
				storage := pickOneStorage(jibuClient, tenant, backupStorage)
				backupStorage = storage.Metadata.Name
				MyBy(fmt.Sprintf("storage is picked, id=%s, display-name=%s", backupStorage, storage.Spec.DisplayName))

				MyBy("create a backup plan")
				meta = swagger.V1ObjectMeta{
					Name: backupPlanName,
				}
				backupPolicy := swagger.V1alpha1BackupPolicy{
					Retention: jobRetention,
					Repeat:    backupRepeatEnabled,
					Frequency: backupFrequency,
				}
				backupPlanSpec := swagger.V1alpha1BackupPlanSpec{
					ClusterName: backupCluster,
					CopyMethod:  backupCopyMethod,
					Desc:        backupPlanName,
					DisplayName: backupPlanName,
					ExcludePV:   !backupWithPV,
					Namespaces:  []string{backupNamespace},
					Policy:      &backupPolicy,
					StorageName: backupStorage,
					Tenant:      tenant,
				}
				backupPlan := swagger.V1alpha1BackupPlan{
					Metadata: &meta,
					Spec:     &backupPlanSpec,
				}
				_, _, err = jibuClient.BackupPlanTagApi.CreateBackupPlan(ctx, tenant, backupPlan)
				Expect(err).ShouldNot(HaveOccurred())
				MyBy(fmt.Sprintf("backup plan %s created", backupPlan.Metadata.Name))

				MyBy(fmt.Sprintf("backup plan should be ready in %v", backupPlanReadyTimeout))
				waitBackupPlanReady(jibuClient, tenant, backupPlanName)
				MyBy("back plan is ready now")

				if !backupRepeatEnabled {
					MyBy("create a backup job")
					meta = swagger.V1ObjectMeta{
						Name: backupJobName,
					}
					backupJobSpec := swagger.V1alpha1BackupJobSpec{
						Action:      actionStartJob,
						BackupName:  backupPlanName,
						Desc:        backupJobName,
						DisplayName: backupJobName,
						Tenant:      tenant,
					}
					backupJob := swagger.V1alpha1BackupJob{
						Metadata: &meta,
						Spec:     &backupJobSpec,
					}
					_, _, err = jibuClient.BackupJobTagApi.CreateBackupJob(ctx, tenant, backupJob)
					Expect(err).ShouldNot(HaveOccurred())
					MyBy(fmt.Sprintf("backup job %s created", backupJobName))

					MyBy(fmt.Sprintf("backup job should complete in %v", backupJobFinishedTimeout))
					waitBackupJobComplete(jibuClient, tenant, backupJobName)
					MyBy("backup job succeeded")
				} else {
					MyBy("wait for repeated creation of backup jobs")

					// indexChan dispatches the index of each backup job
					// sorted by creation time
					// which is retrieved and used by each cronjob execution
					// to find out which job they should be waiting for
					indexChan := make(chan int, backupRepeatCheckNum)
					wg := sync.WaitGroup{}
					checkRepeatedBackupJob := func() {
						var index int
						select {
						case index = <-indexChan:
							defer wg.Done()
						// the last job has not completed yet
						// postpone the check to the next cron scedule
						default:
							return
						}
						MyBy(fmt.Sprintf("wait for backup job to be created in %v, index: %d", backupJobRepeatedCreationTimeout, index))
						jobName := waitNthBackupJobCreation(jibuClient, tenant, backupPlanName, index)
						MyBy(fmt.Sprintf("backup job created, index: %d, name: %s", index, jobName))
						MyBy(fmt.Sprintf("backup job should complete in %v, index: %d, name: %s", backupJobFinishedTimeout, index, jobName))
						waitBackupJobComplete(jibuClient, tenant, jobName)
						MyBy(fmt.Sprintf("backup job completed, index: %d, name: %s", index, jobName))
					}
					c := cron.New()
					c.AddFunc(backupFrequency, checkRepeatedBackupJob)
					c.Start()
					for i := 0; i < backupRepeatCheckNum; i++ {
						indexChan <- i
						wg.Add(1)
						wg.Wait()
					}
					c.Stop()
				}
			}

			if !skipRestore {
				MyBy("pick a cluster for restore")
				cluster := pickOneCluster(jibuClient, tenant, restoreCluster)
				restoreCluster = cluster.Metadata.Name
				MyBy(fmt.Sprintf("cluster is picked, id=%s, display-name=%s", restoreCluster, cluster.Spec.DisplayName))

				MyBy("pick a namespace for restore")
				if restoreNamespace == "" {
					restoreNamespace = determineDestNamespaceName(restoreToSameNamespace, backupNamespace)
				}
				MyBy(fmt.Sprintf("namespace %s is picked", restoreNamespace))

				MyBy("create a restore plan")
				meta = swagger.V1ObjectMeta{
					Name: restorePlanName,
				}
				restorePlanSpec := swagger.V1alpha1RestorePlanSpec{
					BackupName:        backupPlanName,
					Desc:              restorePlanName,
					DestClusterName:   restoreCluster,
					DisplayName:       restorePlanName,
					NamespaceMappings: []string{fmt.Sprintf("%s:%s", backupNamespace, restoreNamespace)},
					Tenant:            tenant,
				}
				restorePlan := swagger.V1alpha1RestorePlan{
					Metadata: &meta,
					Spec:     &restorePlanSpec,
				}
				_, _, err = jibuClient.RestorePlanTagApi.CreateRestorePlan(ctx, tenant, restorePlan)
				Expect(err).ShouldNot(HaveOccurred())
				MyBy(fmt.Sprintf("restore plan %s created", restorePlan.Metadata.Name))

				MyBy("create a restore job")
				meta = swagger.V1ObjectMeta{
					Name: restoreJobName,
				}
				backupJobToRestore, err := pickOneJobOfBackupPlan(jibuClient, tenant, backupPlanName)
				Expect(err).ShouldNot(HaveOccurred())
				restoreJobSpec := swagger.V1alpha1RestoreJobSpec{
					Action:        actionStartJob,
					BackupJobName: backupJobToRestore.Metadata.Name,
					Desc:          restoreJobName,
					DisplayName:   restoreJobName,
					RestoreName:   restorePlanName,
					Tenant:        tenant,
				}
				restoreJob := swagger.V1alpha1RestoreJob{
					Metadata: &meta,
					Spec:     &restoreJobSpec,
				}
				_, _, err = jibuClient.RestoreJobTagApi.CreateRestoreJob(ctx, tenant, restoreJob)
				Expect(err).ShouldNot(HaveOccurred())
				MyBy(fmt.Sprintf("restore job %s created", restoreJob.Metadata.Name))

				MyBy(fmt.Sprintf("restore plan should be ready in %v", restorePlanReadyTimeout))
				waitRestorePlanReady(jibuClient, tenant, restorePlanName)
				MyBy("restore plan is ready now")

				MyBy(fmt.Sprintf("restore job should complete in %v", backupJobFinishedTimeout))
				waitRestoreJobComplete(jibuClient, tenant, restoreJobName)
				MyBy("restore job succeeded")
			}
		})
	})
})

func pickOneCluster(jibuClient *swagger.APIClient, tenant string, cluster string) *swagger.V1alpha1Cluster {
	var c *swagger.V1alpha1Cluster

	if cluster != "" {
		c, _, err := jibuClient.ClusterApi.GetCluster(ctx, tenant, cluster, &swagger.ClusterApiGetClusterOpts{})
		Expect(err).ShouldNot(HaveOccurred())
		return &c
	}

	clusterList, _, err := jibuClient.ClusterApi.ListClusters(ctx, tenant, nil)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(clusterList.Items).ShouldNot(BeEmpty())
	count := 0
	for {
		if count >= resourcePickRetryLimit {
			break
		}
		index := rand.Intn(len(clusterList.Items))
		c = &clusterList.Items[index]
		if c.Status.Phase == string(PhaseReady) {
			break
		} else {
			c = nil
		}
		count++
	}
	Expect(c).ShouldNot(BeNil())
	return c
}

func pickOneStorage(jibuClient *swagger.APIClient, tenant string, storage string) *swagger.V1alpha1Storage {
	var s *swagger.V1alpha1Storage

	if storage != "" {
		s, _, err := jibuClient.StorageApi.GetStorage(ctx, tenant, storage)
		Expect(err).ShouldNot(HaveOccurred())
		return &s
	}

	storageList, _, err := jibuClient.StorageApi.ListStorages(ctx, tenant, nil)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(storageList.Items).ShouldNot(BeEmpty())
	count := 0
	for {
		if count >= resourcePickRetryLimit {
			break
		}
		index := rand.Intn(len(storageList.Items))
		s = &storageList.Items[index]
		if s.Status.Phase == string(PhaseReady) {
			break
		} else {
			s = nil
		}
		count++
	}
	Expect(s).ShouldNot(BeNil())
	return s
}

func pickOneNamespace(jibuClient *swagger.APIClient, tenant string, cluster string, namespace string, excludeNamespaces pie.Strings) *swagger.V1Namespace {
	var ns *swagger.V1Namespace
	nsList, _, err := jibuClient.ClusterApi.GetNamespaces(ctx, tenant, cluster)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(nsList.Items).ShouldNot(BeEmpty())

	if namespace != "" {
		for _, n := range nsList.Items {
			if n.Metadata.Name == namespace {
				ns = &n
				break
			}
		}
		Expect(ns).ShouldNot(BeNil())
		return ns
	}

	count := 0
	for {
		if count >= resourcePickRetryLimit {
			break
		}
		index := rand.Intn(len(nsList.Items))
		ns = &nsList.Items[index]
		if !excludeNamespaces.Contains(ns.Metadata.Name) {
			break
		} else {
			ns = nil
		}
		count++
	}
	Expect(ns).ShouldNot(BeNil())
	return ns
}

func pickOneJobOfBackupPlan(jibuClient *swagger.APIClient, tenant string, planName string) (*swagger.V1alpha1BackupJob, error) {
	listOpts := &swagger.BackupJobTagApiListBackupJobsOpts{
		PlanName:  optional.NewString(planName),
		SortBy:    optional.NewString(FieldCreationTimeStamp),
		Ascending: optional.NewString(strconv.FormatBool(true)),
	}
	jobList, _, err := jibuClient.BackupJobTagApi.ListBackupJobs(ctx, tenant, listOpts)
	if err != nil {
		return nil, err
	}
	if len(jobList.Items) <= 0 {
		return nil, fmt.Errorf("no backup job found")
	}
	return &jobList.Items[0], nil
}

func determineDestNamespaceName(restoreToSameNamespace bool, backupNamespaceName string) string {
	if restoreToSameNamespace {
		return backupNamespaceName
	}
	// namespace name must be within 64 characters
	prefix := backupNamespaceName
	if len(prefix) > 55 {
		prefix = prefix[0:55]
	}
	return strings.ToLower(fmt.Sprintf("%s-%s", prefix, random.GetRandString(5)))
}

func waitBackupPlanReady(jibuClient *swagger.APIClient, tenant string, backupPlanName string) {
	backupPlanReadyCondFunc := func() (bool, error) {
		p, _, err := jibuClient.BackupPlanTagApi.GetBackupPlan(ctx, tenant, backupPlanName)
		if err != nil {
			return false, err
		}
		if p.Status.Phase == string(PhaseReady) {
			return true, nil
		}
		return false, nil
	}
	err := wait.Poll(5*time.Second, backupPlanReadyTimeout, backupPlanReadyCondFunc)
	Expect(err).ShouldNot(HaveOccurred())

	backupPlanCreated, _, err := jibuClient.BackupPlanTagApi.GetBackupPlan(ctx, tenant, backupPlanName)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(backupPlanCreated.Status.Phase).Should(Equal(string(PhaseReady)))
}

func waitRestorePlanReady(jibuClient *swagger.APIClient, tenant string, restorePlanName string) {
	restorePlanReadyCondFunc := func() (bool, error) {
		p, _, err := jibuClient.RestorePlanTagApi.GetRestorePlan(ctx, tenant, restorePlanName)
		if err != nil {
			return false, err
		}
		if p.Status.Phase == string(PhaseReady) {
			return true, nil
		}
		return false, nil
	}
	err := wait.Poll(5*time.Second, restorePlanReadyTimeout, restorePlanReadyCondFunc)
	Expect(err).ShouldNot(HaveOccurred())

	restorePlanCreated, _, err := jibuClient.RestorePlanTagApi.GetRestorePlan(ctx, tenant, restorePlanName)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(restorePlanCreated.Status.Phase).Should(Equal(string(PhaseReady)))
}

func waitBackupJobComplete(jibuClient *swagger.APIClient, tenant string, backupJobName string) {
	backupJobStoppedCondFunc := func() (bool, error) {
		job, _, err := jibuClient.BackupJobTagApi.GetBackupJob(ctx, tenant, backupJobName)
		if err != nil {
			return false, err
		}
		if job.Status.Phase == string(JobPhaseCompleted) || job.Status.Phase == string(JobPhaseFailed) || job.Status.Phase == string(JobPhaseCanceled) {
			return true, nil
		}
		return false, nil
	}
	err := wait.Poll(5*time.Second, backupJobFinishedTimeout, backupJobStoppedCondFunc)
	Expect(err).ShouldNot(HaveOccurred())

	backupJobCreated, _, err := jibuClient.BackupJobTagApi.GetBackupJob(ctx, tenant, backupJobName)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(backupJobCreated.Status.Phase).Should(Equal(string(JobPhaseCompleted)))
}

func waitRestoreJobComplete(jibuClient *swagger.APIClient, tenant string, restoreJobName string) {
	restoreJobStoppedCondFunc := func() (bool, error) {
		job, _, err := jibuClient.RestoreJobTagApi.GetRestoreJob(ctx, tenant, restoreJobName)
		if err != nil {
			return false, err
		}
		if job.Status.Phase == string(JobPhaseCompleted) || job.Status.Phase == string(JobPhaseFailed) || job.Status.Phase == string(JobPhaseCanceled) {
			return true, nil
		}
		return false, nil
	}
	err := wait.Poll(5*time.Second, restoreJobFinishedTimeout, restoreJobStoppedCondFunc)
	Expect(err).ShouldNot(HaveOccurred())

	restoreJobCreated, _, err := jibuClient.RestoreJobTagApi.GetRestoreJob(ctx, tenant, restoreJobName)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(restoreJobCreated.Status.Phase).Should(Equal(string(JobPhaseCompleted)))
}

func getK8sClientFromCluster(jibuClient *swagger.APIClient, tenant string, cluster string) *kubernetes.Clientset {
	// TODO cluster's kubeconfig has been striped, need jibu provide new api
	c, _, err := jibuClient.ClusterApi.GetCluster(ctx, tenant, cluster, &swagger.ClusterApiGetClusterOpts{})
	Expect(err).ShouldNot(HaveOccurred())
	kubeconfigBytes := []byte(c.Spec.Kubeconfig)
	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfigBytes)
	Expect(err).ShouldNot(HaveOccurred())
	restClient, err := clientConfig.ClientConfig()
	Expect(err).ShouldNot(HaveOccurred())
	kubeClient, err := kubernetes.NewForConfig(restClient)
	Expect(err).ShouldNot(HaveOccurred())
	return kubeClient
}

func deleteNamespace(k8sClient *kubernetes.Clientset, namespace string) {
	err := k8sClient.CoreV1().Namespaces().Delete(ctx, namespace, v1.DeleteOptions{})
	Expect(err).ShouldNot(HaveOccurred())
}

func deleteJobsOfBackupPlan(jibuClient *swagger.APIClient, tenant string, planName string) error {
	listOpts := &swagger.BackupJobTagApiListBackupJobsOpts{
		PlanName: optional.NewString(planName),
	}
	var retErr error
	jobList, _, err := jibuClient.BackupJobTagApi.ListBackupJobs(ctx, tenant, listOpts)
	if err != nil {
		retErr = err
	} else {
		for _, j := range jobList.Items {
			_, _, err = jibuClient.BackupJobTagApi.DeleteBackupJob(ctx, tenant, j.Metadata.Name)
			if err != nil {
				retErr = err
			}
		}
	}
	return retErr
}

func waitNthBackupJobCreation(jibuClient *swagger.APIClient, tenant string, planName string, index int) string {
	var jobName string
	listOpts := &swagger.BackupJobTagApiListBackupJobsOpts{
		PlanName:  optional.NewString(planName),
		SortBy:    optional.NewString(FieldCreationTimeStamp),
		Ascending: optional.NewString(strconv.FormatBool(true)),
	}
	nthJobCreatedFunc := func() (bool, error) {
		jobList, _, err := jibuClient.BackupJobTagApi.ListBackupJobs(ctx, tenant, listOpts)
		if err != nil {
			return false, err
		}
		if len(jobList.Items) < index+1 {
			return false, nil
		}
		jobName = jobList.Items[index].Metadata.Name
		return true, nil
	}
	err := wait.Poll(5*time.Second, backupJobRepeatedCreationTimeout, nthJobCreatedFunc)
	Expect(err).ShouldNot(HaveOccurred())
	return jobName
}
