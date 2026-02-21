export namespace types {
	
	export class AppConfig {
	    romm_host: string;
	    username: string;
	    password: string;
	    library_path: string;
	    retroarch_path: string;
	    retroarch_executable: string;
	    cheevos_username: string;
	    cheevos_password: string;
	
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
	        this.cheevos_username = source["cheevos_username"];
	        this.cheevos_password = source["cheevos_password"];
	    }
	}
	export class FileItem {
	    name: string;
	    core: string;
	
	    static createFrom(source: any = {}) {
	        return new FileItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.core = source["core"];
	    }
	}
	export class Game {
	    id: number;
	    name: string;
	    rom_id: number;
	    url_cover: string;
	    full_path: string;
	    summary: string;
	    genres: string[];
	    has_saves: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.rom_id = source["rom_id"];
	        this.url_cover = source["url_cover"];
	        this.full_path = source["full_path"];
	        this.summary = source["summary"];
	        this.genres = source["genres"];
	        this.has_saves = source["has_saves"];
	    }
	}
	export class Platform {
	    id: number;
	    name: string;
	    slug: string;
	    url_icon: string;
	
	    static createFrom(source: any = {}) {
	        return new Platform(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.slug = source["slug"];
	        this.url_icon = source["url_icon"];
	    }
	}

}

