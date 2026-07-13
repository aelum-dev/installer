export namespace main {
	
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
	export class UserAccountData {
	    username: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new UserAccountData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.password = source["password"];
	    }
	}

}

