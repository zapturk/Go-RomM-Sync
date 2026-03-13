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
	    last_used_cores: Record<string, string>;
	    platform_firmware: Record<string, number>;
	    offline_mode: boolean;
	
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
	        this.last_used_cores = source["last_used_cores"];
	        this.platform_firmware = source["platform_firmware"];
	        this.offline_mode = source["offline_mode"];
	    }
	}
	export class FileItem {
	    name: string;
	    core: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new FileItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.core = source["core"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class Firmware {
	    id: number;
	    file_name: string;
	    file_size_bytes: number;
	    is_verified: boolean;
	    platform_id: number;
	    md5_hash: string;
	
	    static createFrom(source: any = {}) {
	        return new Firmware(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.file_name = source["file_name"];
	        this.file_size_bytes = source["file_size_bytes"];
	        this.is_verified = source["is_verified"];
	        this.platform_id = source["platform_id"];
	        this.md5_hash = source["md5_hash"];
	    }
	}
	export class Platform {
	    id: number;
	    name: string;
	    slug: string;
	    url_icon: string;
	    rom_count: number;
	
	    static createFrom(source: any = {}) {
	        return new Platform(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.slug = source["slug"];
	        this.url_icon = source["url_icon"];
	        this.rom_count = source["rom_count"];
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
	    fs_size_bytes: number;
	    platform_id: number;
	    platform_slug: string;
	    platform_display_name: string;
	    platform: Platform;
	    fs_name: string;
	
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
	        this.fs_size_bytes = source["fs_size_bytes"];
	        this.platform_id = source["platform_id"];
	        this.platform_slug = source["platform_slug"];
	        this.platform_display_name = source["platform_display_name"];
	        this.platform = this.convertValues(source["platform"], Platform);
	        this.fs_name = source["fs_name"];
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
	export class LibraryResult_go_romm_sync_types_Game_ {
	    items: Game[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new LibraryResult_go_romm_sync_types_Game_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Game);
	        this.total = source["total"];
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
	export class LibraryResult_go_romm_sync_types_Platform_ {
	    items: Platform[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new LibraryResult_go_romm_sync_types_Platform_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Platform);
	        this.total = source["total"];
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
	
	export class ServerSave {
	    id: number;
	    file_name: string;
	    full_path: string;
	    emulator: string;
	    updated_at: string;
	    file_size_bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerSave(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.file_name = source["file_name"];
	        this.full_path = source["full_path"];
	        this.emulator = source["emulator"];
	        this.updated_at = source["updated_at"];
	        this.file_size_bytes = source["file_size_bytes"];
	    }
	}
	export class ServerState {
	    id: number;
	    file_name: string;
	    full_path: string;
	    emulator: string;
	    updated_at: string;
	    file_size_bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.file_name = source["file_name"];
	        this.full_path = source["full_path"];
	        this.emulator = source["emulator"];
	        this.updated_at = source["updated_at"];
	        this.file_size_bytes = source["file_size_bytes"];
	    }
	}

}

