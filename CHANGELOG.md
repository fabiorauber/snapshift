# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.5] - 2026-05-20

### Fixed
- PVC creation from snapshot now uses the snapshot's `RestoreSize` instead of the source PVC size, fixing `snapshot size mismatch` errors with vSphere CSI (and other drivers that require the PVC request to exactly match the snapshot size)

## [0.1.4] - 2026-05-08

### Fixed
- Destination PVC not bound after migration is now a fatal error instead of a warning; the tool no longer reports success when the PVC remains in `Pending` phase
- "Snapshots deleted" summary line is no longer printed when snapshot cleanup failed

### Changed
- Default `--timeout` increased from `10m` to `1h` to accommodate large data migrations

## [0.1.3] - 2026-04-22

### Fixed
- Destination PVC is now created using the source PVC's actual bound capacity instead of the requested size, avoiding under-provisioning when the provisioner rounded up the volume

## [0.1.2] - 2025-12-09

### Added
- `--delete-snapshots` flag to automatically remove snapshots after PVC creation

## [0.1.1] - 2025-12-09

### Added
- Automatic cleanup of created resources on operation failure
- Default destination PVC name to match source PVC name (no longer required to specify `--dest-pvc-name`)
- `--create-namespace` flag to automatically create destination namespace if it doesn't exist

## [0.1.0] - 2025-12-09

### Added
- Initial release of SnapShift
- Support for creating snapshots of PVCs in origin cluster
- Replication of snapshots to destination cluster using same snapshotHandle
- Optional PVC creation from replicated snapshots
- Support for multiple kubeconfig files
- Context switching for both origin and destination clusters
- Configurable timeout for snapshot operations
- Custom snapshot naming
- Support for VolumeSnapshotClass specification
- Comprehensive CLI with cobra framework
- Detailed logging and progress feedback
- Core snapshot migration functionality
- Basic CLI interface
- Support for Kubernetes 1.17+ with CSI snapshots
- Integration with external-snapshotter v6

### Documentation
- README with installation and usage instructions
- QUICKSTART guide for getting started in 5 minutes
- EXAMPLES with common usage scenarios
- ARCHITECTURE documentation explaining design decisions
- Contributing guidelines

[Unreleased]: https://github.com/fabiorauber/snapshift/compare/v0.1.5...HEAD
[0.1.5]: https://github.com/fabiorauber/snapshift/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/fabiorauber/snapshift/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/fabiorauber/snapshift/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/fabiorauber/snapshift/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/fabiorauber/snapshift/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/fabiorauber/snapshift/releases/tag/v0.1.0
[0.1.0]: https://github.com/fabiorauber/snapshift/releases/tag/v0.1.0
