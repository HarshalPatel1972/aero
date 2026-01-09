export namespace main {
	
	export class NetworkInterface {
	    name: string;
	    ip: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkInterface(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ip = source["ip"];
	    }
	}
	export class ServerStatus {
	    running: boolean;
	    url: string;
	    ip: string;
	    port: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.url = source["url"];
	        this.ip = source["ip"];
	        this.port = source["port"];
	    }
	}

}

