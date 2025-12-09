# Quick Start Guide

Get up and running with SnapShift in 5 minutes.

## Installation

```bash
# Clone the repository
git clone https://github.com/fabiorauber/snapshift
cd snapshift

# Build the binary
go build -o snapshift

# (Optional) Install to PATH
sudo cp snapshift /usr/local/bin/
```

Or use Go install:

```bash
go install github.com/fabiorauber/snapshift@latest
```

## Prerequisites Check

Before running snapshift, verify your environment:

```bash
# 1. Check you have access to both clusters
kubectl config get-contexts

# 2. Verify VolumeSnapshotClass exists
kubectl get volumesnapshotclass

# 3. Confirm CSI driver is installed
kubectl get csidrivers

# 4. Check your source PVC exists
kubectl get pvc -n <namespace>
```

## Basic Usage

### 1. Simple Snapshot Migration

Snapshot a PVC and replicate to another cluster:

```bash
snapshift \
  --origin-context cluster-a \
  --dest-context cluster-b \
  --pvc my-data \
  --namespace default
```

### 2. Snapshot + Restore

Create a snapshot and immediately restore it as a new PVC:

```bash
snapshift \
  --origin-context cluster-a \
  --dest-context cluster-b \
  --pvc my-data \
  --namespace default \
  --create-pvc \
  --dest-pvc-name my-data-restored
```

### 3. Cross-Namespace Migration

Migrate PVC to a different namespace:

```bash
snapshift \
  --origin-context production \
  --dest-context staging \
  --pvc app-storage \
  --namespace prod-app \
  --create-pvc \
  --dest-pvc-name app-storage \
  --dest-namespace staging-app
```

## Step-by-Step Example

Let's walk through a complete example:

### Scenario

You have:
- Origin cluster: `prod-cluster` with PVC `postgres-data` in namespace `database`
- Destination cluster: `dr-cluster`
- Goal: Create a backup in DR cluster

### Step 1: Verify Source PVC

```bash
kubectl --context prod-cluster get pvc postgres-data -n database
```

Output:
```
NAME            STATUS   VOLUME          CAPACITY   ACCESS MODES   STORAGECLASS
postgres-data   Bound    pvc-abc123...   50Gi       RWO            fast-storage
```

### Step 2: Run SnapShift

```bash
snapshift \
  --origin-context prod-cluster \
  --dest-context dr-cluster \
  --pvc postgres-data \
  --namespace database \
  --snapshot-name postgres-dr-backup-20231215
```

### Step 3: Monitor Progress

The tool will show progress:

```
Connecting to origin cluster...
Connecting to destination cluster...
Fetching PVC database/postgres-data from origin cluster...
Found PVC with size: 50Gi
Creating snapshot database/postgres-dr-backup-20231215 in origin cluster...
Waiting for origin snapshot to be ready...
  Snapshot status: ReadyToUse=false
  Snapshot status: ReadyToUse=true
Fetching VolumeSnapshotContent snapcontent-12345...
Origin snapshot handle: csi-snapshot-abc-xyz-123
Creating VolumeSnapshotContent in destination cluster...
Created VolumeSnapshotContent: snapcontent-postgres-dr-backup-20231215
Creating VolumeSnapshot database/postgres-dr-backup-20231215 in destination cluster...
Waiting for destination snapshot to be ready...
  Snapshot status: ReadyToUse=true
Destination snapshot is ready!

✓ Successfully completed snapshot migration!
  Origin snapshot: database/postgres-dr-backup-20231215
  Destination snapshot: database/postgres-dr-backup-20231215
```

### Step 4: Verify in Destination

```bash
kubectl --context dr-cluster get volumesnapshot -n database
```

Output:
```
NAME                              READYTOUSE   SOURCEPVC   RESTORESIZE
postgres-dr-backup-20231215       true                     50Gi
```

### Step 5 (Optional): Create PVC from Snapshot

When needed, restore the data:

```bash
snapshift \
  --origin-context prod-cluster \
  --dest-context dr-cluster \
  --pvc postgres-data \
  --namespace database \
  --snapshot-name postgres-dr-backup-20231215 \
  --create-pvc \
  --dest-pvc-name postgres-data-restored
```

## Common Options

### Specify VolumeSnapshotClass

```bash
snapshift \
  --pvc my-data \
  --namespace default \
  --origin-context origin \
  --dest-context dest \
  --snapshot-class fast-snapclass
```

### Use Different Kubeconfig Files

```bash
snapshift \
  --origin-kubeconfig ~/.kube/cluster1-config \
  --dest-kubeconfig ~/.kube/cluster2-config \
  --pvc my-data \
  --namespace default
```

### Increase Timeout for Large PVCs

```bash
snapshift \
  --pvc large-volume \
  --namespace default \
  --origin-context origin \
  --dest-context dest \
  --timeout 30m
```

## Troubleshooting

### Error: "failed to get source PVC: not found"

**Solution**: Check PVC name and namespace:

```bash
kubectl get pvc -n <namespace>
```

### Error: "timeout waiting for snapshot to be ready"

**Solution**: Increase timeout or check CSI driver:

```bash
# Increase timeout
snapshift --pvc <name> --timeout 30m ...

# Check CSI driver
kubectl logs -n kube-system -l app=csi-snapshotter
```

### Error: "failed to create destination VolumeSnapshotContent"

**Solution**: Verify CSI driver compatibility and storage access:

```bash
# Check CSI driver in both clusters
kubectl get csidrivers

# Verify it's the same driver
kubectl get volumesnapshotcontent -o yaml | grep driver
```

### Warning: Snapshot created but data seems old

**Issue**: Both clusters must access the same storage backend.

**Verification**:
```bash
# Check if storage classes point to same backend
kubectl get storageclass -o yaml
```

## Next Steps

Now that you've completed the quick start:

1. Read [EXAMPLES.md](EXAMPLES.md) for more usage scenarios
2. Check [ARCHITECTURE.md](ARCHITECTURE.md) to understand how it works
3. Review [README.md](README.md) for complete flag reference
4. Join our community for support and discussions

## Getting Help

- **Documentation**: See [README.md](README.md) for detailed information
- **Examples**: Check [EXAMPLES.md](EXAMPLES.md) for common scenarios
- **Issues**: Report bugs on GitHub Issues
- **Questions**: Use GitHub Discussions

## What's Next?

Try these common scenarios:

1. **DR Setup**: Automate regular snapshots to DR cluster
2. **Test Refresh**: Clone production data to test environment
3. **Cluster Migration**: Move all PVCs to a new cluster
4. **Backup Strategy**: Implement multi-cluster backup workflow

Happy snapshot migrating! 🚀
