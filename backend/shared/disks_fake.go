package shared

// Fake disk and partition data for UI testing.
// Set FAKE_DISKS=1 to use this data instead of querying lsblk.

func fakeDisks() []DiskInfo {
	return []DiskInfo{
		{Name: "nvme0n1", Path: "/dev/nvme0n1", Size: "512110984960", SizeFormatted: "476.9 GB", Type: "disk", Model: "Samsung SSD 980 PRO 500GB", Vendor: "Samsung", Rotational: false, Readonly: false, Trans: "nvme", IsRemovable: false, Serial: "S6YANF0R123456"},
		{Name: "vda", Path: "/dev/vda", Size: "42949672960", SizeFormatted: "40.0 GB", Type: "disk", Model: "QEMU HARDDISK 40GB", Vendor: "QEMU", Rotational: false, Readonly: false, Trans: "virtio", IsRemovable: false, Serial: "QEMU_QM00001"},
		{Name: "sda", Path: "/dev/sda", Size: "1000204886016", SizeFormatted: "931.5 GB", Type: "disk", Model: "WD Blue SA516 1TB", Vendor: "Western Digital", Rotational: true, Readonly: false, Trans: "sata", Fstype: "ext4", Mountpoint: "/", Serial: "WDBJBK001001"},
		{Name: "sdb", Path: "/dev/sdb", Size: "2000398934016", SizeFormatted: "1.8 TB", Type: "disk", Model: "Seagate Barracuda 2TB", Vendor: "Seagate", Rotational: true, Readonly: false, Trans: "sata", Fstype: "ntfs", Mountpoint: "", Serial: "ZTK8F9A1"},
		{Name: "nvme1n1", Path: "/dev/nvme1n1", Size: "2000398934016", SizeFormatted: "1.8 TB", Type: "disk", Model: "Samsung SSD 870 EVO 2TB", Vendor: "Samsung", Rotational: false, Readonly: false, Trans: "nvme", IsRemovable: false, Serial: "S5RBNX0H987654"},
		{Name: "sdc", Path: "/dev/sdc", Size: "12803567616", SizeFormatted: "11.9 GB", Type: "disk", Model: "Verbatim STORE N GO", Vendor: "Verbatim", Rotational: true, Readonly: false, Trans: "usb", IsRemovable: true, Serial: "VB12061605000123"},
		{Name: "sdd", Path: "/dev/sdd", Size: "500107862016", SizeFormatted: "465.8 GB", Type: "disk", Model: "Crucial MX500 500GB", Vendor: "Crucial", Rotational: false, Readonly: false, Trans: "sata", Fstype: "btrfs", Mountpoint: "/home", Serial: "2112E9234567"},
		{Name: "sde", Path: "/dev/sde", Size: "4000787031616", SizeFormatted: "3.6 TB", Type: "disk", Model: "WD Red Plus 4TB", Vendor: "Western Digital", Rotational: true, Readonly: false, Trans: "sata", IsRemovable: false, Serial: "WD-WMC3T0123456"},
		{Name: "nvme2n1", Path: "/dev/nvme2n1", Size: "1000215250688", SizeFormatted: "931.5 GB", Type: "disk", Model: "SK hynix Platinum P41 1TB", Vendor: "SK hynix", Rotational: false, Readonly: false, Trans: "nvme", IsRemovable: false, Serial: "HEPQGH812345"},
		{Name: "sdg", Path: "/dev/sdg", Size: "32000000000", SizeFormatted: "29.8 GB", Type: "disk", Model: "SanDisk Ultra Fit", Vendor: "SanDisk", Rotational: true, Readonly: false, Trans: "usb", IsRemovable: true, Serial: "43431202345678901234"},
		{Name: "sdh", Path: "/dev/sdh", Size: "8001567940096", SizeFormatted: "7.3 TB", Type: "disk", Model: "Toshiba X300 8TB", Vendor: "Toshiba", Rotational: true, Readonly: false, Trans: "sata", Fstype: "ext4", Mountpoint: "/mnt/data", Serial: "81VY2A123"},
	}
}

func fakePartitions(diskPath string) []PartitionDetail {
	switch diskPath {
	case "/dev/nvme0n1":
		return []PartitionDetail{
			{Path: "/dev/nvme0n1p1", Name: "nvme0n1p1", Type: "part", Size: "536870912", SizeFormatted: "512.0 MB", Fstype: "fat32", UUID: "A1B2-C3D4", Mountpoints: []string{"/boot/efi"}},
			{Path: "/dev/nvme0n1p2", Name: "nvme0n1p2", Type: "part", Size: "511574094848", SizeFormatted: "476.4 GB", Fstype: "ext4", UUID: "e5f6-a7b8-c9d0-e1f2", Mountpoints: []string{}},
		}
	case "/dev/vda":
		return []PartitionDetail{
			{Path: "/dev/vda1", Name: "vda1", Type: "part", Size: "536870912", SizeFormatted: "512.0 MB", Fstype: "fat32", UUID: "VDA1-EFI", Mountpoints: []string{"/boot/efi"}},
			{Path: "/dev/vda2", Name: "vda2", Type: "part", Size: "42412802048", SizeFormatted: "39.5 GB", Fstype: "ext4", UUID: "VDA2-ROOT", Mountpoints: []string{}},
		}
	case "/dev/sda":
		return []PartitionDetail{
			{Path: "/dev/sda1", Name: "sda1", Type: "part", Size: "536870912", SizeFormatted: "512.0 MB", Fstype: "fat32", UUID: "B2C3-D4E5", Mountpoints: []string{"/boot/efi"}},
			{Path: "/dev/sda2", Name: "sda2", Type: "part", Size: "42949672960", SizeFormatted: "40.0 GB", Fstype: "ext4", UUID: "c3d4-e5f6-a7b8-c9d0", Mountpoints: []string{"/"}},
			{Path: "/dev/sda3", Name: "sda3", Type: "part", Size: "926655841792", SizeFormatted: "863.0 GB", Fstype: "ext4", UUID: "d4e5-f6a7-b8c9-d0e1", Mountpoints: []string{"/home"}},
			{Path: "/dev/sda4", Name: "sda4", Type: "part", Size: "60344669184", SizeFormatted: "56.2 GB", Fstype: "swap", UUID: "e5f6-a7b8-c9d0-e1f2", Mountpoints: []string{"[SWAP]"}},
		}
	case "/dev/sdb":
		return []PartitionDetail{{Path: "/dev/sdb1", Name: "sdb1", Type: "part", Size: "2000398934016", SizeFormatted: "1.8 TB", Fstype: "ntfs", UUID: "F0A1-B2C3-D4E5-F6A7", Mountpoints: []string{}}}
	case "/dev/nvme1n1":
		return []PartitionDetail{
			{Path: "/dev/nvme1n1p1", Name: "nvme1n1p1", Type: "part", Size: "536870912", SizeFormatted: "512.0 MB", Fstype: "fat32", UUID: "C3D4-E5F6", Mountpoints: []string{"/boot/efi"}},
			{Path: "/dev/nvme1n1p2", Name: "nvme1n1p2", Type: "part", Size: "1999861872640", SizeFormatted: "1.8 TB", Fstype: "btrfs", UUID: "a7b8-c9d0-e1f2-a3b4", Mountpoints: []string{}},
		}
	case "/dev/sdc":
		return []PartitionDetail{{Path: "/dev/sdc1", Name: "sdc1", Type: "part", Size: "12803567616", SizeFormatted: "11.9 GB", Fstype: "vfat", UUID: "D4E5-F6A7", Mountpoints: []string{}}}
	case "/dev/sdd":
		return []PartitionDetail{{Path: "/dev/sdd1", Name: "sdd1", Type: "part", Size: "500107862016", SizeFormatted: "465.8 GB", Fstype: "btrfs", UUID: "b8c9-d0e1-f2a3-b4c5", Mountpoints: []string{"/home"}}}
	case "/dev/sde":
		return []PartitionDetail{
			{Path: "/dev/sde1", Name: "sde1", Type: "part", Size: "536870912", SizeFormatted: "512.0 MB", Fstype: "fat32", UUID: "E5F6-A7B8", Mountpoints: []string{"/boot/efi"}},
			{Path: "/dev/sde2", Name: "sde2", Type: "part", Size: "3999649161728", SizeFormatted: "3.6 TB", Fstype: "ext4", UUID: "c9d0-e1f2-a3b4-c5d6", Mountpoints: []string{}},
		}
	case "/dev/nvme2n1":
		return []PartitionDetail{
			{Path: "/dev/nvme2n1p1", Name: "nvme2n1p1", Type: "part", Size: "536870912", SizeFormatted: "512.0 MB", Fstype: "fat32", UUID: "F6A7-B8C9", Mountpoints: []string{"/boot/efi"}},
			{Path: "/dev/nvme2n1p2", Name: "nvme2n1p2", Type: "part", Size: "999678379776", SizeFormatted: "931.0 GB", Fstype: "ext4", UUID: "d0e1-f2a3-b4c5-d6e7", Mountpoints: []string{}},
		}
	case "/dev/sdg":
		return []PartitionDetail{{Path: "/dev/sdg1", Name: "sdg1", Type: "part", Size: "32000000000", SizeFormatted: "29.8 GB", Fstype: "vfat", UUID: "A7B8-C9D0", Mountpoints: []string{}}}
	case "/dev/sdh":
		return []PartitionDetail{
			{Path: "/dev/sdh1", Name: "sdh1", Type: "part", Size: "536870912", SizeFormatted: "512.0 MB", Fstype: "fat32", UUID: "B8C9-D0E1", Mountpoints: []string{"/boot/efi"}},
			{Path: "/dev/sdh2", Name: "sdh2", Type: "part", Size: "7999649161728", SizeFormatted: "7.3 TB", Fstype: "ext4", UUID: "e1f2-a3b4-c5d6-e7f8", Mountpoints: []string{"/mnt/data"}},
		}
	default:
		return []PartitionDetail{}
	}
}
