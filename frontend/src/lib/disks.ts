import type { DiskInfo, PartitionEntry } from '$lib/types';
import { FileSystemBtrfs, FileSystemExt4 } from '$lib/constants';

/**
 * Best-effort filesystem for the auto-install filesystem pill, used when the
 * disk itself doesn't report one. Prefers the partition mounted at "/", then a
 * btrfs/ext4 partition, then any formatted partition. Returns null when none
 * of the disk's partitions carry a filesystem.
 */
export function inferDiskFileSystem(
	disk: DiskInfo,
	partitions: PartitionEntry[],
): string | null {
	if (disk.fstype) return disk.fstype;

	const rootPartition = partitions.find((p) => p.mountpoints?.includes('/'));
	if (rootPartition?.fstype) return rootPartition.fstype;

	const nativePartition = partitions.find(
		(p) => p.fstype === FileSystemBtrfs || p.fstype === FileSystemExt4,
	);
	if (nativePartition?.fstype) return nativePartition.fstype;

	return partitions.find((p) => p.fstype)?.fstype ?? null;
}
