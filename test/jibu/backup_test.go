package jibu

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/stoneshi-yunify/jibutest/pkg/utils/random"
	swagger "github.com/jibutech/backup-saas-client"
	"k8s.io/apimachinery/pkg/util/wait"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBackupAndRestore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "backup and restore")
}

var _ = Describe("use jibu api", func() {
	rand.Seed(time.Now().UnixNano())

	jibuConf := swagger.NewConfiguration()
	jibuConf.BasePath = jibuAPIEndpoint
	jibuClient := swagger.NewAPIClient(jibuConf)

	var backupClusterID string
	var restoreClusterID string
	var restoredNamespace string

	Context("create a backup job and a restore job", func() {
		BeforeEach(func() {
			By("clean up at the beginning")
			_, _, _ = jibuClient.BackupPlanTagApi.DeleteBackupPlan(ctx, tenant, backupPlanName)
			_, _, _ = jibuClient.BackupJobTagApi.DeleteBackupJob(ctx, tenant, backupJobName)
			_, _, _ = jibuClient.RestorePlanTagApi.DeleteRestorePlan(ctx, tenant, restorePlanName)
			_, _, _ = jibuClient.RestoreJobTagApi.DeleteRestoreJob(ctx, tenant, restoreJobName)
		})

		AfterEach(func() {
			By("clean up at the end")
			if !restoreToSameNamespace || backupClusterID != restoreClusterID {
				if len(restoreClusterID) != 0 && len(restoredNamespace) != 0 {
					k8sClient := getK8sClientFromCluster(jibuClient, restoreClusterID)
					deleteNamespace(k8sClient, restoredNamespace)
				}
			}
		})

		It("should succeed", func() {
			var err error

			By("pick a random cluster for backup")
			cluster := randomPickOneCluster(jibuClient)
			backupClusterID = cluster.Metadata.Name
			By(fmt.Sprintf("cluster is picked, id=%s, display-name=%s", cluster.Metadata.Name, cluster.Spec.DisplayName))

			By("pick a random namespace")
			ns := randomPickOneNamespace(jibuClient, cluster.Metadata.Name)
			By(fmt.Sprintf("namespace %s is picked", ns.Metadata.Name))

			By("pick a random storage")
			storage := randomPickOneStorage(jibuClient)
			By(fmt.Sprintf("storage is picked, id=%s, display-name=%s", storage.Metadata.Name, storage.Spec.DisplayName))

			By("create a backup plan")
			meta := swagger.V1ObjectMeta{
				Name: backupPlanName,
			}
			backupPolicy := swagger.V1alpha1BackupPolicy{
				Retention: jobRetention,
			}
			backupPlanSpec := swagger.V1alpha1BackupPlanSpec{
				ClusterName: cluster.Metadata.Name,
				Desc:        backupPlanName,
				DisplayName: backupPlanName,
				ExcludePV:   !backupWithPV,
				Namespaces:  []string{ns.Metadata.Name},
				Policy:      &backupPolicy,
				StorageName: storage.Metadata.Name,
				Tenant:      tenant,
			}
			backupPlan := swagger.V1alpha1BackupPlan{
				Metadata: &meta,
				Spec:     &backupPlanSpec,
			}
			_, _, err = jibuClient.BackupPlanTagApi.CreateBackupPlan(ctx, tenant, backupPlan)
			Expect(err).ShouldNot(HaveOccurred())
			By(fmt.Sprintf("backup plan %s created", backupPlan.Metadata.Name))

			By("create a backup job")
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
			By(fmt.Sprintf("backup job %s created", backupJobName))

			By(fmt.Sprintf("backup plan should be ready in %v", backupPlanReadyTimeout))
			waitBackupPlanReady(jibuClient)
			By("back plan is ready now")

			By(fmt.Sprintf("backup job should complete in %v", backupJobFinishedTimeout))
			waitBackupJobComplete(jibuClient)
			By("backup job succeeded")

			By("pick a random cluster for restore")
			restoreCluster := randomPickOneCluster(jibuClient)
			restoreClusterID = restoreCluster.Metadata.Name
			By(fmt.Sprintf("cluster is picked, id=%s, display-name=%s", restoreCluster.Metadata.Name, restoreCluster.Spec.DisplayName))

			By("pick a namespace for restore")
			restoredNamespace = determineDestNamespaceName(ns.Metadata.Name)
			By(fmt.Sprintf("namespace %s is picked", restoredNamespace))

			By("create a restore plan")
			meta = swagger.V1ObjectMeta{
				Name: restorePlanName,
			}
			restorePlanSpec := swagger.V1alpha1RestorePlanSpec{
				BackupName:        backupPlanName,
				Desc:              restorePlanName,
				DestClusterName:   restoreCluster.Metadata.Name,
				DisplayName:       restorePlanName,
				NamespaceMappings: []string{fmt.Sprintf("%s:%s", ns.Metadata.Name, restoredNamespace)},
				Tenant:            tenant,
			}
			restorePlan := swagger.V1alpha1RestorePlan{
				Metadata: &meta,
				Spec:     &restorePlanSpec,
			}
			_, _, err = jibuClient.RestorePlanTagApi.CreateRestorePlan(ctx, tenant, restorePlan)
			Expect(err).ShouldNot(HaveOccurred())
			By(fmt.Sprintf("restore plan %s created", restorePlan.Metadata.Name))

			By("create a restore job")
			meta = swagger.V1ObjectMeta{
				Name: restoreJobName,
			}
			restoreJobSpec := swagger.V1alpha1RestoreJobSpec{
				Action:        actionStartJob,
				BackupJobName: backupJobName,
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
			By(fmt.Sprintf("restore job %s created", restoreJob.Metadata.Name))

			By(fmt.Sprintf("restore plan should be ready in %v", restorePlanReadyTimeout))
			waitRestorePlanReady(jibuClient)
			By("restore plan is ready now")

			By(fmt.Sprintf("restore job should complete in %v", backupJobFinishedTimeout))
			waitRestoreJobComplete(jibuClient)
			By("restore job succeeded")
		})
	})
})

func randomPickOneCluster(jibuClient *swagger.APIClient) *swagger.V1alpha1Cluster {
	clusterList, _, err := jibuClient.ClusterApi.ListClusters(ctx, tenant, nil)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(clusterList.Items).ShouldNot(BeEmpty())
	var cluster *swagger.V1alpha1Cluster
	count := 0
	for {
		if count >= resourcePickRetryLimit {
			break
		}
		index := rand.Intn(len(clusterList.Items))
		cluster = &clusterList.Items[index]
		if cluster.Status.Phase == string(PhaseReady) {
			break
		}
		count++
	}
	Expect(cluster).ShouldNot(BeNil())
	return cluster
}

func randomPickOneStorage(jibuClient *swagger.APIClient) *swagger.V1alpha1Storage {
	storageList, _, err := jibuClient.StorageApi.ListStorages(ctx, tenant, nil)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(storageList.Items).ShouldNot(BeEmpty())
	var storage *swagger.V1alpha1Storage
	count := 0
	for {
		if count >= resourcePickRetryLimit {
			break
		}
		index := rand.Intn(len(storageList.Items))
		storage = &storageList.Items[index]
		if storage.Status.Phase == string(PhaseReady) {
			break
		}
		count++
	}
	Expect(storage).ShouldNot(BeNil())
	return storage
}

func randomPickOneNamespace(jibuClient *swagger.APIClient, cluster string) *swagger.V1Namespace {
	nsList, _, err := jibuClient.ClusterApi.GetNamespaces(ctx, tenant, cluster)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(nsList.Items).ShouldNot(BeEmpty())
	var ns *swagger.V1Namespace
	count := 0
	for {
		if count >= resourcePickRetryLimit {
			break
		}
		index := rand.Intn(len(nsList.Items))
		ns = &nsList.Items[index]
		if !excludeNamespaces.Contains(ns.Metadata.Name) {
			break
		}
		count++
	}
	Expect(ns).ShouldNot(BeNil())
	return ns
}

func determineDestNamespaceName(backupNamespaceName string) string {
	if restoreToSameNamespace {
		return backupNamespaceName
	}
	return strings.ToLower(fmt.Sprintf("restore-%s-%s", backupNamespaceName, random.GetRandString(5)))
}

func waitBackupPlanReady(jibuClient *swagger.APIClient) {
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

func waitRestorePlanReady(jibuClient *swagger.APIClient) {
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

func waitBackupJobComplete(jibuClient *swagger.APIClient) {
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

func waitRestoreJobComplete(jibuClient *swagger.APIClient) {
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

func getK8sClientFromCluster(jibuClient *swagger.APIClient, cluster string) *kubernetes.Clientset {
	c, _, err := jibuClient.ClusterApi.GetCluster(ctx, tenant, cluster)
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
