export namespace core {
	
	export class Installer {
	    Logger: any;
	
	    static createFrom(source: any = {}) {
	        return new Installer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Logger = source["Logger"];
	    }
	}
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

export namespace shared {
	
	export class DiskInfo {
	    name: string;
	    path: string;
	    size: string;
	    type: string;
	    model: string;
	    vendor: string;
	    rotational: boolean;
	    ro: boolean;
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
	        this.type = source["type"];
	        this.model = source["model"];
	        this.vendor = source["vendor"];
	        this.rotational = source["rotational"];
	        this.ro = source["ro"];
	        this.fstype = source["fstype"];
	        this.mountpoint = source["mountpoint"];
	        this.serial = source["serial"];
	        this.trans = source["trans"];
	        this.isRemovable = source["isRemovable"];
	    }
	}
	export class DiskSelectionData {
	    diskId: string;
	    size: string;
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
	export class PartitionDetail {
	    path: string;
	    name: string;
	    type: string;
	    size: string;
	    fstype: string;
	    uuid: string;
	    mountpoints: string[];
	
	    static createFrom(source: any = {}) {
	        return new PartitionDetail(source);
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

