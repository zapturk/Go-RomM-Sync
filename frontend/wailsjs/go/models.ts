export namespace types {
	
	export class AppConfig {
	    romm_host: string;
	    username: string;
	    password: string;
	    library_path: string;
	    retroarch_path: string;
	    retroarch_executable: string;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.romm_host = source["romm_host"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.library_path = source["library_path"];
	        this.retroarch_path = source["retroarch_path"];
	        this.retroarch_executable = source["retroarch_executable"];
	    }
	}

}

