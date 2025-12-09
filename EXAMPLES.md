# Example Usage Scenarios

This document provides detailed examples for common use cases of `snapshift`.

## Prerequisites Verification

Before using snapshift, verify your environment:

```bash
# Check VolumeSnapshotClass exists
kubectl get volumesnapshotclass

# Check CSI driver supports snapshots
kubectl get csidrivers

# Verify your PVC
kubectl get pvc -n <namespace>

# Check snapshot CRDs are installed
kubectl get crd | grep snapshot
```

## Scenario 1: Simple Cross-Cluster Snapshot

Create a snapshot in one cluster and make it available in another:

```bash
snapshift \
  --origin-context prod-us-east \
  --dest-context prod-us-west \
  --pvc mysql-data \
  --namespace database \
  --snapshot-class csi-cephfs-snapclass
```

**What happens:**
1. Creates snapshot `mysql-data-snapshot-<timestamp>` in prod-us-east
2. Waits for snapshot to be ready
3. Extracts the snapshotHandle from the underlying storage
4. Creates VolumeSnapshotContent in prod-us-west with same snapshotHandle
5. Creates VolumeSnapshot in prod-us-west bound to the content

## Scenario 2: DR Restore

Disaster recovery: snapshot production data and restore in DR cluster:

```bash
# Step 1: Create snapshot and replicate to DR
snapshift \
  --origin-kubeconfig ~/.kube/prod-config \
  --dest-kubeconfig ~/.kube/dr-config \
  --pvc app-data \
  --namespace production \
  --snapshot-name dr-backup-$(date +%Y%m%d-%H%M%S) \
  --timeout 30m

# Step 2: When needed, create PVC from snapshot in DR
snapshift \
  --origin-kubeconfig ~/.kube/prod-config \
  --dest-kubeconfig ~/.kube/dr-config \
  --pvc app-data \
  --namespace production \
  --create-pvc \
  --dest-namespace production \
  --delete-snapshots \
  --snapshot-name dr-backup-20231215-143022
```

## Scenario 3: Blue/Green Deployment

Copy data from blue environment to green:

```bash
snapshift \
  --origin-context prod-blue \
  --dest-context prod-green \
  --pvc application-state \
  --namespace myapp \
  --create-pvc \
  --dest-namespace myapp
```

## Scenario 4: Test Environment Refresh

Clone production data to testing:

```bash
# Refresh test environment with latest production data
snapshift \
  --origin-context production \
  --dest-context testing \
  --pvc user-uploads \
  --namespace app \
  --create-pvc \
  --dest-pvc-name user-uploads-test \
  --dest-namespace test-app \
  --snapshot-name test-refresh-$(date +%Y%m%d)
```

## Scenario 5: Multi-PVC Migration

Script to migrate multiple PVCs:

```bash
#!/bin/bash

PVCS=(
  "database-data:postgres"
  "app-storage:myapp"
  "cache-volume:redis"
)

for pvc_entry in "${PVCS[@]}"; do
  IFS=':' read -r pvc namespace <<< "$pvc_entry"
  
  echo "Migrating $pvc from $namespace..."
  
  snapshift \
    --origin-context old-cluster \
    --dest-context new-cluster \
    --pvc "$pvc" \
    --namespace "$namespace" \
    --create-pvc \
    --dest-namespace "$namespace" \
    --create-namespace \
    --timeout 20m
  
  if [ $? -eq 0 ]; then
    echo "✓ Successfully migrated $pvc"
  else
    echo "✗ Failed to migrate $pvc"
  fi
done
```

## Scenario 6: Custom Snapshot Names

Use specific naming for compliance:

```bash
snapshift \
  --origin-context production \
  --dest-context backup \
  --pvc financial-data \
  --namespace finance \
  --snapshot-name "financial-data-monthly-backup-2023-12" \
  --dest-snapshot-name "financial-data-monthly-backup-2023-12" \
  --snapshot-class encrypted-snapclass \
  --create-pvc \
  --dest-namespace finance-backup \
  --create-namespace
```

## Scenario 7: Cross-Region Replication

Replicate data across regions (same storage backend):

```bash
# Regions: us-east-1 → us-west-2
snapshift \
  --origin-kubeconfig ~/.kube/us-east-1 \
  --dest-kubeconfig ~/.kube/us-west-2 \
  --pvc critical-app-data \
  --namespace apps \
  --snapshot-class fast-snapshot \
  --timeout 45m
```

## Scenario 8: Complete Migration with Cleanup

Migrate data and automatically clean up snapshots:

```bash
snapshift \
  --origin-context old-cluster \
  --dest-context new-cluster \
  --pvc application-data \
  --namespace myapp \
  --create-pvc \
  --create-namespace \
  --delete-snapshots
```

**What happens:**
1. Creates snapshot in old-cluster
2. Replicates to new-cluster
3. Creates PVC from snapshot
4. Deletes both snapshots automatically
5. Leaves only the new PVC with data

## Scenario 9: Namespace Isolation

Copy PVC to a different namespace in the same or different cluster:

```bash
snapshift \
  --origin-context shared-cluster \
  --dest-context shared-cluster \
  --pvc shared-data \
  --namespace team-a \
  --create-pvc \
  --dest-namespace team-b \
  --create-namespace
```

## Error Recovery

### If snapshot creation fails:

```bash
# Check the snapshot status
kubectl describe volumesnapshot <snapshot-name> -n <namespace>

# Check CSI driver logs
kubectl logs -n kube-system -l app=csi-snapshotter

# Retry with increased timeout
snapshift --pvc <pvc-name> --namespace <ns> --timeout 30m
```

### If PVC creation fails:

```bash
# Verify snapshot is ready in destination
kubectl get volumesnapshot -n <namespace>

# Check if snapshot has RestoreSize
kubectl get volumesnapshot <snapshot-name> -n <namespace> -o yaml

# Ensure StorageClass exists
kubectl get storageclass
```

## Performance Considerations

### Large PVCs

For PVCs > 100GB:

```bash
snapshift \
  --pvc large-dataset \
  --namespace data \
  --timeout 60m \  # Increase timeout
  --origin-context origin \
  --dest-context dest
```

### Multiple Snapshots

Use GNU parallel for concurrent migrations:

```bash
parallel -j 3 snapshift \
  --origin-context origin \
  --dest-context dest \
  --pvc {} \
  --namespace default \
  --create-pvc \
  --dest-pvc-name {}-migrated ::: pvc1 pvc2 pvc3 pvc4 pvc5
```

## Monitoring Progress

Track snapshot operations:

```bash
# In terminal 1 - Origin cluster
watch kubectl get volumesnapshot,volumesnapshotcontent -A

# In terminal 2 - Destination cluster  
watch kubectl get volumesnapshot,volumesnapshotcontent -A

# In terminal 3 - Run snapshift
snapshift --pvc mydata --namespace default \
  --origin-context origin --dest-context dest \
  --create-pvc --dest-pvc-name mydata-restored
```

## Cleanup

Remove snapshots after successful migration:

```bash
# Origin cluster
kubectl delete volumesnapshot <snapshot-name> -n <namespace>

# Destination cluster (if needed)
kubectl delete volumesnapshot <snapshot-name> -n <namespace>

# VolumeSnapshotContents with DeletionPolicy: Retain must be manually deleted
kubectl delete volumesnapshotcontent <content-name>
```
