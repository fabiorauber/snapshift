package main

import (
	"context"
	"fmt"
	"os"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	snapshotclient "github.com/kubernetes-csi/external-snapshotter/client/v6/clientset/versioned"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	originKubeconfig string
	destKubeconfig   string
	originContext    string
	destContext      string
	pvcName          string
	pvcNamespace     string
	snapshotName     string
	destSnapshotName string
	createPVC        bool
	destPVCName      string
	destNamespace    string
	snapshotClass    string
	timeout          time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "snapshift",
	Short: "Snapshot and migrate PVCs across Kubernetes clusters",
	Long: `snapshift is a CLI tool that creates a snapshot of a PVC in an origin cluster,
replicates the snapshot to a destination cluster (using the same underlying storage),
and optionally creates a PVC from the snapshot in the destination cluster.`,
	RunE: runSnapshift,
}

func init() {
	rootCmd.Flags().StringVar(&originKubeconfig, "origin-kubeconfig", "", "Path to origin cluster kubeconfig (defaults to KUBECONFIG or ~/.kube/config)")
	rootCmd.Flags().StringVar(&destKubeconfig, "dest-kubeconfig", "", "Path to destination cluster kubeconfig (defaults to same as origin)")
	rootCmd.Flags().StringVar(&originContext, "origin-context", "", "Origin cluster context name")
	rootCmd.Flags().StringVar(&destContext, "dest-context", "", "Destination cluster context name")
	rootCmd.Flags().StringVarP(&pvcName, "pvc", "p", "", "Name of the PVC to snapshot (required)")
	rootCmd.Flags().StringVarP(&pvcNamespace, "namespace", "n", "default", "Namespace of the source PVC")
	rootCmd.Flags().StringVar(&snapshotName, "snapshot-name", "", "Name for the snapshot (defaults to <pvc-name>-snapshot-<timestamp>)")
	rootCmd.Flags().StringVar(&destSnapshotName, "dest-snapshot-name", "", "Name for destination snapshot (defaults to same as origin)")
	rootCmd.Flags().BoolVar(&createPVC, "create-pvc", false, "Create a PVC from the snapshot in destination cluster")
	rootCmd.Flags().StringVar(&destPVCName, "dest-pvc-name", "", "Name for the destination PVC (required if --create-pvc is set)")
	rootCmd.Flags().StringVar(&destNamespace, "dest-namespace", "", "Destination namespace (defaults to same as source)")
	rootCmd.Flags().StringVar(&snapshotClass, "snapshot-class", "", "VolumeSnapshotClass name (optional, uses default if not specified)")
	rootCmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Timeout for snapshot operations")

	if err := rootCmd.MarkFlagRequired("pvc"); err != nil {
		panic(fmt.Sprintf("failed to mark pvc flag as required: %v", err))
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runSnapshift(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Set defaults
	if snapshotName == "" {
		snapshotName = fmt.Sprintf("%s-snapshot-%d", pvcName, time.Now().Unix())
	}
	if destSnapshotName == "" {
		destSnapshotName = snapshotName
	}
	if destNamespace == "" {
		destNamespace = pvcNamespace
	}
	if createPVC && destPVCName == "" {
		return fmt.Errorf("--dest-pvc-name is required when --create-pvc is set")
	}

	// Create origin cluster clients
	fmt.Printf("Connecting to origin cluster...\n")
	originK8sClient, originSnapClient, err := createClients(originKubeconfig, originContext)
	if err != nil {
		return fmt.Errorf("failed to create origin cluster clients: %w", err)
	}

	// Create destination cluster clients
	fmt.Printf("Connecting to destination cluster...\n")
	destK8sClient, destSnapClient, err := createClients(destKubeconfig, destContext)
	if err != nil {
		return fmt.Errorf("failed to create destination cluster clients: %w", err)
	}

	// Track created resources for cleanup on failure
	var (
		originSnapshotCreated = false
		destContentCreated    = false
		destSnapshotCreated   = false
		destContentName       string
	)

	// Step 1: Get source PVC
	fmt.Printf("Fetching PVC %s/%s from origin cluster...\n", pvcNamespace, pvcName)
	sourcePVC, err := originK8sClient.CoreV1().PersistentVolumeClaims(pvcNamespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get source PVC: %w", err)
	}
	storageSize := sourcePVC.Spec.Resources.Requests[corev1.ResourceStorage]
	fmt.Printf("Found PVC with size: %s\n", storageSize.String())

	// Step 2: Create snapshot in origin cluster
	fmt.Printf("Creating snapshot %s/%s in origin cluster...\n", pvcNamespace, snapshotName)
	_, err = createSnapshot(ctx, originSnapClient, pvcNamespace, snapshotName, pvcName, snapshotClass)
	if err != nil {
		return fmt.Errorf("failed to create origin snapshot: %w", err)
	}
	originSnapshotCreated = true

	// Setup cleanup on failure
	defer func() {
		if err != nil {
			cleanupOnFailure(context.Background(), originSnapClient, destSnapClient,
				originSnapshotCreated, pvcNamespace, snapshotName,
				destContentCreated, destContentName,
				destSnapshotCreated, destNamespace, destSnapshotName)
		}
	}()

	// Step 3: Wait for origin snapshot to be ready
	fmt.Printf("Waiting for origin snapshot to be ready...\n")
	originSnapshot, err := waitForSnapshotReady(ctx, originSnapClient, pvcNamespace, snapshotName)
	if err != nil {
		return fmt.Errorf("failed waiting for origin snapshot: %w", err)
	}

	if originSnapshot.Status == nil || originSnapshot.Status.BoundVolumeSnapshotContentName == nil {
		return fmt.Errorf("origin snapshot does not have a bound VolumeSnapshotContent")
	}

	// Step 4: Get the VolumeSnapshotContent from origin
	fmt.Printf("Fetching VolumeSnapshotContent %s...\n", *originSnapshot.Status.BoundVolumeSnapshotContentName)
	originContent, err := originSnapClient.SnapshotV1().VolumeSnapshotContents().Get(ctx, *originSnapshot.Status.BoundVolumeSnapshotContentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get origin VolumeSnapshotContent: %w", err)
	}

	if originContent.Status == nil || originContent.Status.SnapshotHandle == nil {
		return fmt.Errorf("origin VolumeSnapshotContent does not have a snapshot handle")
	}
	snapshotHandle := *originContent.Status.SnapshotHandle
	fmt.Printf("Found snapshot handle: %s\n", snapshotHandle)

	// Step 5: Create VolumeSnapshotContent in destination cluster (with same snapshotHandle)
	fmt.Printf("Creating VolumeSnapshotContent in destination cluster...\n")
	destContentName = fmt.Sprintf("snapcontent-%s", destSnapshotName)
	destContent, err := createVolumeSnapshotContent(ctx, destSnapClient, destContentName, destNamespace, destSnapshotName, snapshotHandle, originContent)
	if err != nil {
		return fmt.Errorf("failed to create destination VolumeSnapshotContent: %w", err)
	}
	destContentCreated = true
	fmt.Printf("Created VolumeSnapshotContent: %s\n", destContent.Name)

	// Step 6: Create VolumeSnapshot in destination cluster (pre-bound to the content)
	fmt.Printf("Creating VolumeSnapshot %s/%s in destination cluster...\n", destNamespace, destSnapshotName)
	_, err = createPreBoundSnapshot(ctx, destSnapClient, destNamespace, destSnapshotName, destContentName, snapshotClass)
	if err != nil {
		return fmt.Errorf("failed to create destination snapshot: %w", err)
	}
	destSnapshotCreated = true
	// Step 7: Wait for destination snapshot to be ready
	fmt.Printf("Waiting for destination snapshot to be ready...\n")
	_, err = waitForSnapshotReady(ctx, destSnapClient, destNamespace, destSnapshotName)
	if err != nil {
		return fmt.Errorf("failed waiting for destination snapshot: %w", err)
	}
	fmt.Printf("Destination snapshot is ready!\n")
	fmt.Printf("Destination snapshot is ready!\n")

	// Step 8: Optionally create PVC from snapshot
	if createPVC {
		fmt.Printf("Creating PVC %s/%s from snapshot...\n", destNamespace, destPVCName)
		pvc, err := createPVCFromSnapshot(ctx, destK8sClient, destNamespace, destPVCName, destSnapshotName, sourcePVC)
		if err != nil {
			return fmt.Errorf("failed to create destination PVC: %w", err)
		}
		fmt.Printf("Created PVC: %s/%s\n", pvc.Namespace, pvc.Name)
	}

	fmt.Printf("\n✓ Successfully completed snapshot migration!\n")
	fmt.Printf("  Origin snapshot: %s/%s\n", pvcNamespace, snapshotName)
	fmt.Printf("  Destination snapshot: %s/%s\n", destNamespace, destSnapshotName)
	if createPVC {
		fmt.Printf("  Destination PVC: %s/%s\n", destNamespace, destPVCName)
	}

	return nil
}

func createClients(kubeconfigPath, contextName string) (*kubernetes.Clientset, *snapshotclient.Clientset, error) {
	// Load kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		configOverrides.CurrentContext = contextName
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Create Kubernetes clientset
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Create snapshot clientset
	snapClient, err := snapshotclient.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create snapshot client: %w", err)
	}

	return k8sClient, snapClient, nil
}

func createSnapshot(ctx context.Context, client *snapshotclient.Clientset, namespace, name, pvcName, snapshotClass string) (*snapshotv1.VolumeSnapshot, error) {
	snapshot := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvcName,
			},
		},
	}

	if snapshotClass != "" {
		snapshot.Spec.VolumeSnapshotClassName = &snapshotClass
	}

	return client.SnapshotV1().VolumeSnapshots(namespace).Create(ctx, snapshot, metav1.CreateOptions{})
}

func createVolumeSnapshotContent(ctx context.Context, client *snapshotclient.Clientset, name, namespace, snapshotName, snapshotHandle string, originContent *snapshotv1.VolumeSnapshotContent) (*snapshotv1.VolumeSnapshotContent, error) {
	// Create a pre-provisioned VolumeSnapshotContent
	content := &snapshotv1.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: snapshotv1.VolumeSnapshotContentSpec{
			VolumeSnapshotRef: corev1.ObjectReference{
				Name:      snapshotName,
				Namespace: namespace,
			},
			Source: snapshotv1.VolumeSnapshotContentSource{
				SnapshotHandle: &snapshotHandle,
			},
			Driver:         originContent.Spec.Driver,
			DeletionPolicy: snapshotv1.VolumeSnapshotContentRetain, // Use Retain to keep the underlying snapshot
		},
	}

	// Copy VolumeSnapshotClassName if present
	if originContent.Spec.VolumeSnapshotClassName != nil {
		content.Spec.VolumeSnapshotClassName = originContent.Spec.VolumeSnapshotClassName
	}

	return client.SnapshotV1().VolumeSnapshotContents().Create(ctx, content, metav1.CreateOptions{})
}

func createPreBoundSnapshot(ctx context.Context, client *snapshotclient.Clientset, namespace, name, contentName, snapshotClass string) (*snapshotv1.VolumeSnapshot, error) {
	snapshot := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				VolumeSnapshotContentName: &contentName,
			},
		},
	}

	if snapshotClass != "" {
		snapshot.Spec.VolumeSnapshotClassName = &snapshotClass
	}

	return client.SnapshotV1().VolumeSnapshots(namespace).Create(ctx, snapshot, metav1.CreateOptions{})
}

func waitForSnapshotReady(ctx context.Context, client *snapshotclient.Clientset, namespace, name string) (*snapshotv1.VolumeSnapshot, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for snapshot to be ready")
		case <-ticker.C:
			snapshot, err := client.SnapshotV1().VolumeSnapshots(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}

			if snapshot.Status != nil && snapshot.Status.ReadyToUse != nil && *snapshot.Status.ReadyToUse {
				return snapshot, nil
			}

			if snapshot.Status != nil && snapshot.Status.Error != nil {
				return nil, fmt.Errorf("snapshot error: %s", *snapshot.Status.Error.Message)
			}

			fmt.Printf("  Snapshot status: ReadyToUse=%v\n", snapshot.Status != nil && snapshot.Status.ReadyToUse != nil && *snapshot.Status.ReadyToUse)
		}
	}
}

func createPVCFromSnapshot(ctx context.Context, client *kubernetes.Clientset, namespace, pvcName, snapshotName string, sourcePVC *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	// Get the storage size from source PVC
	storageSize := sourcePVC.Spec.Resources.Requests[corev1.ResourceStorage]

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: sourcePVC.Spec.AccessModes,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageSize,
				},
			},
			DataSource: &corev1.TypedLocalObjectReference{
				APIGroup: stringPtr("snapshot.storage.k8s.io"),
				Kind:     "VolumeSnapshot",
				Name:     snapshotName,
			},
		},
	}

	// Copy storage class if present
	if sourcePVC.Spec.StorageClassName != nil {
		pvc.Spec.StorageClassName = sourcePVC.Spec.StorageClassName
	}

	return client.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
}

func stringPtr(s string) *string {
	return &s
}

func cleanupOnFailure(ctx context.Context, originSnapClient, destSnapClient *snapshotclient.Clientset,
	originSnapshotCreated bool, originNamespace, originSnapshotName string,
	destContentCreated bool, destContentName string,
	destSnapshotCreated bool, destNamespace, destSnapshotName string) {

	fmt.Printf("\n⚠ Operation failed, cleaning up created resources...\n")

	// Clean up destination snapshot
	if destSnapshotCreated {
		fmt.Printf("  Deleting destination snapshot %s/%s...\n", destNamespace, destSnapshotName)
		err := destSnapClient.SnapshotV1().VolumeSnapshots(destNamespace).Delete(ctx, destSnapshotName, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("  ✗ Failed to delete destination snapshot: %v\n", err)
		} else {
			fmt.Printf("  ✓ Deleted destination snapshot\n")
		}
	}

	// Clean up destination snapshot content
	if destContentCreated {
		fmt.Printf("  Deleting destination VolumeSnapshotContent %s...\n", destContentName)
		err := destSnapClient.SnapshotV1().VolumeSnapshotContents().Delete(ctx, destContentName, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("  ✗ Failed to delete destination VolumeSnapshotContent: %v\n", err)
		} else {
			fmt.Printf("  ✓ Deleted destination VolumeSnapshotContent\n")
		}
	}

	// Clean up origin snapshot
	if originSnapshotCreated {
		fmt.Printf("  Deleting origin snapshot %s/%s...\n", originNamespace, originSnapshotName)
		err := originSnapClient.SnapshotV1().VolumeSnapshots(originNamespace).Delete(ctx, originSnapshotName, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("  ✗ Failed to delete origin snapshot: %v\n", err)
		} else {
			fmt.Printf("  ✓ Deleted origin snapshot\n")
		}
	}
	fmt.Printf("Cleanup completed.\n\n")
}
