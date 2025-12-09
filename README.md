# SnapShift

A command-line utility for snapshotting and migrating PersistentVolumeClaims (PVCs) across Kubernetes clusters that share the same underlying storage system.

## Overview

`snapshift` enables you to:
1. Create a snapshot of a PVC in an origin Kubernetes cluster
2. Replicate the snapshot to a destination Kubernetes cluster using the same `snapshotHandle`
3. Optionally create a new PVC from the snapshot in the destination cluster

This is particularly useful for disaster recovery, cluster migration, or data replication scenarios where both clusters have access to the same storage backend (e.g., the same Ceph cluster, cloud storage, or SAN).

## Prerequisites

- Go 1.21 or later
- Access to both origin and destination Kubernetes clusters
- Both clusters must use the same underlying storage system
- CSI driver with snapshot support installed on both clusters
- VolumeSnapshotClass configured on both clusters
- Appropriate RBAC permissions for:
  - Reading PVCs in the origin cluster
  - Creating VolumeSnapshots and VolumeSnapshotContents in both clusters
  - Creating PVCs in the destination cluster (if using `--create-pvc`)

## Installation

### From Source

```bash
git clone https://github.com/fabiorauber/snapshift
cd snapshift
go mod download
go build -o snapshift
```

### Quick Install

```bash
go install github.com/fabiorauber/snapshift@latest
```

## Usage

### Basic Snapshot Migration

Snapshot a PVC and replicate it to the destination cluster:

```bash
snapshift \
  --origin-context origin-cluster \
  --dest-context dest-cluster \
  --pvc my-pvc \
  --namespace default
```

### Complete Migration with PVC Creation

Snapshot, replicate, and create a new PVC in the destination:

```bash
snapshift \
  --origin-context origin-cluster \
  --dest-context dest-cluster \
  --pvc my-pvc \
  --namespace default \
  --create-pvc \
  --dest-pvc-name restored-pvc \
  --dest-namespace production
```

### Using Different Kubeconfig Files

```bash
snapshift \
  --origin-kubeconfig ~/.kube/cluster1-config \
  --dest-kubeconfig ~/.kube/cluster2-config \
  --pvc my-pvc \
  --namespace default \
  --create-pvc \
  --dest-pvc-name my-restored-pvc
```

### Specify VolumeSnapshotClass

```bash
snapshift \
  --origin-context origin-cluster \
  --dest-context dest-cluster \
  --pvc my-pvc \
  --namespace default \
  --snapshot-class csi-snapclass \
  --create-pvc \
  --dest-pvc-name restored-pvc
```

## Command-Line Flags

| Flag | Description | Required | Default |
|------|-------------|----------|---------|
| `--pvc`, `-p` | Name of the PVC to snapshot | Yes | - |
| `--namespace`, `-n` | Namespace of the source PVC | No | `default` |
| `--origin-kubeconfig` | Path to origin cluster kubeconfig | No | `$KUBECONFIG` or `~/.kube/config` |
| `--dest-kubeconfig` | Path to destination cluster kubeconfig | No | Same as origin |
| `--origin-context` | Origin cluster context name | No | Current context |
| `--dest-context` | Destination cluster context name | No | Current context |
| `--snapshot-name` | Name for the snapshot | No | `<pvc-name>-snapshot-<timestamp>` |
| `--dest-snapshot-name` | Name for destination snapshot | No | Same as origin |
| `--snapshot-class` | VolumeSnapshotClass name | No | Uses default class |
| `--create-pvc` | Create a PVC from snapshot in destination | No | `false` |
| `--dest-pvc-name` | Name for the destination PVC | Required if `--create-pvc` | - |
| `--dest-namespace` | Destination namespace | No | Same as source |
| `--timeout` | Timeout for snapshot operations | No | `10m` |

## How It Works

1. **Connect to Clusters**: Establishes connections to both origin and destination clusters using kubeconfig files
2. **Fetch Source PVC**: Retrieves the source PVC metadata from the origin cluster
3. **Create Origin Snapshot**: Creates a VolumeSnapshot in the origin cluster
4. **Wait for Snapshot**: Waits for the origin snapshot to become ready
5. **Extract SnapshotHandle**: Retrieves the `snapshotHandle` from the VolumeSnapshotContent
6. **Create Destination Content**: Creates a VolumeSnapshotContent in the destination cluster with the same `snapshotHandle`
7. **Create Destination Snapshot**: Creates a pre-bound VolumeSnapshot in the destination cluster
8. **Wait for Ready**: Waits for the destination snapshot to become ready
9. **Create PVC** (optional): Creates a new PVC from the snapshot in the destination cluster

## Architecture Requirements

Both Kubernetes clusters must:
- Use CSI drivers from the same storage backend
- Have access to the same underlying storage system
- Support VolumeSnapshots (Kubernetes 1.17+)
- Have the external-snapshotter components installed

## Examples

### Example 1: DR Scenario

Create a snapshot in production and restore it in DR cluster:

```bash
# Create snapshot and restore to DR cluster
snapshift \
  --origin-context prod-cluster \
  --dest-context dr-cluster \
  --pvc database-data \
  --namespace postgres \
  --create-pvc \
  --dest-pvc-name database-data-restored \
  --dest-namespace postgres-dr
```

### Example 2: Testing with Production Data

Snapshot production data and create a test PVC:

```bash
snapshift \
  --origin-context production \
  --dest-context staging \
  --pvc app-storage \
  --namespace production \
  --create-pvc \
  --dest-pvc-name app-storage-test \
  --dest-namespace testing \
  --snapshot-class fast-snapclass
```

### Example 3: Cluster Migration

Migrate all PVCs from one cluster to another:

```bash
# For each PVC, run:
for pvc in $(kubectl get pvc -n myapp -o jsonpath='{.items[*].metadata.name}'); do
  snapshift \
    --origin-context old-cluster \
    --dest-context new-cluster \
    --pvc "$pvc" \
    --namespace myapp \
    --create-pvc \
    --dest-pvc-name "$pvc" \
    --dest-namespace myapp
done
```

## Troubleshooting

### Snapshot Stays in Pending State

- Check that the VolumeSnapshotClass exists and is properly configured
- Verify the CSI driver supports snapshots
- Check CSI driver logs for errors

### SnapshotHandle Not Found

- Ensure the snapshot is fully ready before proceeding
- Verify the VolumeSnapshotContent was created successfully
- Check CSI driver compatibility between clusters

### PVC Creation Fails

- Verify the snapshot is ready in the destination cluster
- Check that the StorageClass exists in the destination cluster
- Ensure sufficient storage quota is available

### Permission Errors

Required RBAC permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: snapshift-user
rules:
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "create"]
- apiGroups: ["snapshot.storage.k8s.io"]
  resources: ["volumesnapshots", "volumesnapshotcontents"]
  verbs: ["get", "list", "create"]
```

## Limitations

- Both clusters must share the same underlying storage system
- The CSI driver must support the same snapshot format on both clusters
- VolumeSnapshotClass configurations should be compatible
- Network latency may affect operation timeout requirements

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.