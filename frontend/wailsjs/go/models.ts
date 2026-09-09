export namespace gui {
	
	export class PageID {
	    Value: number;
	
	    static createFrom(source: any = {}) {
	        return new PageID(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Value = source["Value"];
	    }
	}

}

export namespace install {
	
	export class RootCheckResult {
	    authorized: boolean;
	    error?: string;
	    message: string;
	    output?: string;
	
	    static createFrom(source: any = {}) {
	        return new RootCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.authorized = source["authorized"];
	        this.error = source["error"];
	        this.message = source["message"];
	        this.output = source["output"];
	    }
	}

}

export namespace shared {
	
	export class DiskInfo {
	    name: string;
	    path: string;
	    size: number;
	    sizeFormatted: string;
	    type: string;
	    model: string;
	    vendor: string;
	    rotational: boolean;
	    readonly: boolean;
	    fstype: string;
	    mountpoint: string;
	    serial: string;
	    trans: string;
	    isRemovable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DiskInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.sizeFormatted = source["sizeFormatted"];
	        this.type = source["type"];
	        this.model = source["model"];
	        this.vendor = source["vendor"];
	        this.rotational = source["rotational"];
	        this.readonly = source["readonly"];
	        this.fstype = source["fstype"];
	        this.mountpoint = source["mountpoint"];
	        this.serial = source["serial"];
	        this.trans = source["trans"];
	        this.isRemovable = source["isRemovable"];
	    }
	}
	export class DiskSelectionData {
	    diskId: string;
	    size: number;
	    fileSystem: string;
	
	    static createFrom(source: any = {}) {
	        return new DiskSelectionData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.diskId = source["diskId"];
	        this.size = source["size"];
	        this.fileSystem = source["fileSystem"];
	    }
	}
	export class Language {
	    code: string;
	    name: string;
	    locale: string;
	
	    static createFrom(source: any = {}) {
	        return new Language(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.locale = source["locale"];
	    }
	}
	export class ManualPartition {
	    index: number;
	    path: string;
	    sizeMB: number;
	    type: string;
	    sizeBytes: number;
	    existingFstype?: string;
	
	    static createFrom(source: any = {}) {
	        return new ManualPartition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.path = source["path"];
	        this.sizeMB = source["sizeMB"];
	        this.type = source["type"];
	        this.sizeBytes = source["sizeBytes"];
	        this.existingFstype = source["existingFstype"];
	    }
	}
	export class ManualLayout {
	    disk: string;
	    diskSizeMB: number;
	    diskSizeBytes: number;
	    partitions: ManualPartition[];
	    FreeMB: number;
	    freeSectors: number;
	    deletedPartitions?: ManualPartition[];
	
	    static createFrom(source: any = {}) {
	        return new ManualLayout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.disk = source["disk"];
	        this.diskSizeMB = source["diskSizeMB"];
	        this.diskSizeBytes = source["diskSizeBytes"];
	        this.partitions = this.convertValues(source["partitions"], ManualPartition);
	        this.FreeMB = source["FreeMB"];
	        this.freeSectors = source["freeSectors"];
	        this.deletedPartitions = this.convertValues(source["deletedPartitions"], ManualPartition);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class PartitionEntry {
	    path: string;
	    name: string;
	    type: string;
	    size: number;
	    fstype: string;
	    uuid: string;
	    mountpoints: string[];
	    
	    static createFrom(source: any = {}) {
	        return new PartitionEntry(source);
	    }
	    
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.size = source["size"];
	        this.fstype = source["fstype"];
	        this.uuid = source["uuid"];
	        this.mountpoints = source["mountpoints"];
	    }
	}
	export class UserAccountData {
	    username: string;
	    password: string;
	    confirmPassword: string;
	
	    static createFrom(source: any = {}) {
	        return new UserAccountData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.password = source["password"];
	        this.confirmPassword = source["confirmPassword"];
	    }
	}

}

