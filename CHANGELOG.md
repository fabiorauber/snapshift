# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Automatic cleanup of created resources on operation failure

### Fixed
- Fixed errcheck linter error by properly handling `MarkFlagRequired` return value
- Fixed cleanup of VolumeSnapshotContent in destination cluster when operation fails

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

[0.1.0]: https://github.com/fabiorauber/snapshift/releases/tag/v0.1.0
