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

