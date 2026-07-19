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
	    }
	}
	export class DiskSelectionData {
	    diskId: string;
	
	    static createFrom(source: any = {}) {
	        return new DiskSelectionData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.diskId = source["diskId"];
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

