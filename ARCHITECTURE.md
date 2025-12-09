# Architecture and Design

## Overview

SnapShift is designed to facilitate PVC snapshot migration across Kubernetes clusters that share the same underlying storage system. This document explains the architecture, design decisions, and technical details.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         SnapShift CLI                            │
└────────────┬──────────────────────────────────┬─────────────────┘
             │                                  │
             ▼                                  ▼
    ┌────────────────┐                 ┌────────────────┐
    │ Origin Cluster │                 │  Dest Cluster  │
    │   K8s Client   │                 │   K8s Client   │
    └────────┬───────┘                 └────────┬───────┘
             │                                  │
             ▼                                  ▼
    ┌────────────────┐                 ┌────────────────┐
    │      PVC       │                 │  New PVC (opt) │
    │       │        │                 │       │        │
    │       ▼        │                 │       ▼        │
    │ VolumeSnapshot │                 │ VolumeSnapshot │
    │       │        │                 │       │        │
    │       ▼        │                 │       ▼        │
    │ SnapshotContent│────────┐        │ SnapshotContent│
    │  (snapshotID)  │        │        │  (snapshotID)  │
    └────────────────┘        │        └────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │ Shared Storage   │
                    │  (Ceph, NetApp,  │
                    │   Cloud Storage) │
                    └──────────────────┘
```

## Key Components

### 1. Client Management

**Purpose**: Establish connections to both Kubernetes clusters.

```go
func createClients(kubeconfigPath, contextName string) 
    (*kubernetes.Clientset, *snapshotclient.Clientset, error)
```

- Loads kubeconfig from file or default location
- Supports context switching
- Creates both standard k8s client and snapshot-specific client
- Reuses same function for origin and destination clusters

### 2. Snapshot Creation

**Purpose**: Create and wait for snapshot in origin cluster.

**Flow**:
1. Create VolumeSnapshot referencing the source PVC
2. CSI driver creates actual snapshot in storage backend
3. VolumeSnapshotContent is created with snapshotHandle
4. Poll until snapshot.status.readyToUse = true

```go
func createSnapshot(ctx, client, namespace, name, pvcName, snapshotClass)
func waitForSnapshotReady(ctx, client, namespace, name) 
```

### 3. Snapshot Replication

**Purpose**: Create snapshot objects in destination using same storage snapshot.

**Critical Design**: The snapshotHandle is the key that links both clusters to the same underlying storage snapshot.

**Flow**:
1. Retrieve VolumeSnapshotContent from origin
2. Extract snapshotHandle (CSI-specific identifier)
3. Create VolumeSnapshotContent in destination with:
   - Same snapshotHandle
   - Same CSI driver name
   - Pre-bound to destination VolumeSnapshot
4. Create VolumeSnapshot in destination referencing the content
5. Wait for snapshot to become ready

```go
func createVolumeSnapshotContent(...)
func createPreBoundSnapshot(...)
```

### 4. PVC Restoration (Optional)

**Purpose**: Create a new PVC from the replicated snapshot.

**Flow**:
1. Create PVC with dataSource pointing to VolumeSnapshot
2. CSI driver provisions PV from snapshot
3. PVC is bound and ready to use

```go
func createPVCFromSnapshot(...)
```

## Design Decisions

### 1. Pre-bound Snapshots

**Decision**: Use pre-bound VolumeSnapshotContent in destination cluster.

**Rationale**:
- Ensures the destination snapshot uses the exact same storage snapshot
- Prevents CSI driver from creating a new snapshot
- Maintains data consistency between clusters

**Implementation**:
```go
content := &snapshotv1.VolumeSnapshotContent{
    Spec: snapshotv1.VolumeSnapshotContentSpec{
        VolumeSnapshotRef: corev1.ObjectReference{
            Name:      snapshotName,
            Namespace: namespace,
        },
        Source: snapshotv1.VolumeSnapshotContentSource{
            SnapshotHandle: &snapshotHandle, // Same as origin
        },
        DeletionPolicy: snapshotv1.VolumeSnapshotContentRetain,
    },
}
```

### 2. Retain Deletion Policy

**Decision**: Use `Retain` deletion policy for VolumeSnapshotContent.

**Rationale**:
- Prevents accidental deletion of underlying storage snapshot
- Allows snapshot to be used by multiple clusters
- Users must explicitly delete the storage snapshot when no longer needed

### 3. Polling for Readiness

**Decision**: Poll snapshot status every 5 seconds with configurable timeout.

**Rationale**:
- CSI snapshot creation is asynchronous
- Different storage backends have different performance characteristics
- Provides user feedback during long operations

### 4. CLI-based Tool

**Decision**: Command-line tool vs operator or controller.

**Rationale**:
- Simpler deployment (single binary)
- No cluster-wide installation required
- Easier for ad-hoc operations
- User has full control over timing

## Storage Backend Requirements

The tool requires both clusters to:

1. **Use the same CSI driver**: Same driver name in VolumeSnapshotContent
2. **Access the same storage**: Physical or logical access to same storage backend
3. **Support snapshots**: CSI driver must implement snapshot capabilities
4. **Share snapshot namespace**: SnapshotHandle must be valid in both clusters

### Compatible Storage Systems

- **Ceph RBD/CephFS**: Clusters connecting to same Ceph cluster
- **NetApp**: Shared NetApp backend
- **Cloud Storage**: 
  - AWS EBS (with cross-region snapshot copy)
  - Azure Disk (with cross-region snapshot)
  - Google Persistent Disk (with cross-zone access)
- **Portworx**: Clusters in same Portworx fabric
- **StorageOS**: Clusters with shared StorageOS cluster
- **vSphere CNS**: Clusters using the same vSAN backend

## Failure Modes and Recovery

### Origin Snapshot Fails

**Symptom**: Snapshot never becomes ready in origin cluster.

**Recovery**:
```bash
kubectl describe volumesnapshot <name> -n <namespace>
kubectl logs -n kube-system <csi-driver-pod>
```

**Tool Behavior**: Returns error, no objects created in destination.

### Destination Content Creation Fails

**Symptom**: VolumeSnapshotContent creation fails in destination.

**Possible Causes**:
- Different CSI driver version
- Storage not accessible from destination
- Invalid snapshotHandle format

**Recovery**:
- Verify CSI driver compatibility
- Check storage connectivity
- Manually delete partial objects

### Timeout Exceeded

**Symptom**: Context deadline exceeded error.

**Recovery**:
```bash
# Increase timeout
snapshift --pvc <name> --timeout 30m ...
```

**Tool Behavior**: Objects may be partially created; check cluster state.

## Security Considerations

### RBAC Requirements

**Origin Cluster**:
- `get` on PersistentVolumeClaims
- `create, get, list` on VolumeSnapshots
- `get` on VolumeSnapshotContents

**Destination Cluster**:
- `create, get, list` on VolumeSnapshots
- `create, get` on VolumeSnapshotContents
- `create` on PersistentVolumeClaims (if --create-pvc)

### Kubeconfig Security

- Store kubeconfig files securely
- Use context-specific credentials
- Consider service accounts with minimal permissions
- Rotate credentials regularly

### Data Security

- Snapshots inherit storage encryption settings
- Network communication uses TLS (Kubernetes API)
- Consider encryption at rest for storage backend

## Performance Characteristics

### Time Complexity

- **Snapshot Creation**: O(1) API calls + storage backend time
- **Snapshot Replication**: O(1) API calls (metadata only)
- **PVC Restoration**: O(1) API calls + storage provisioning time

### Network Usage

- Minimal network traffic (API calls only)
- No data transfer between clusters
- Data stays on shared storage backend

### Storage Backend Impact

- **Origin**: One snapshot creation
- **Destination**: No new storage snapshot
- **Shared Storage**: Single snapshot used by both clusters

## Future Enhancements

### Planned Features

1. **Batch Operations**: Migrate multiple PVCs in one command
2. **Progress Bar**: Visual feedback for long operations
3. **Retry Logic**: Automatic retry with exponential backoff
4. **Dry Run Mode**: Preview operations without execution
5. **Snapshot Validation**: Verify data integrity after migration
6. **Label/Annotation Copying**: Preserve metadata from source PVC

## Contributing

When contributing to snapshift, please:

1. **Update documentation** for API changes
2. **Follow Go conventions** (gofmt, golint)
3. **Add examples** for new features
4. **Update CHANGELOG** with your changes

## References

- [Kubernetes Volume Snapshots](https://kubernetes.io/docs/concepts/storage/volume-snapshots/)
- [CSI Specification](https://github.com/container-storage-interface/spec)
- [External Snapshotter](https://github.com/kubernetes-csi/external-snapshotter)
- [Volume Snapshot Best Practices](https://kubernetes.io/blog/2020/12/10/kubernetes-1.20-volume-snapshot-moves-to-ga/)
