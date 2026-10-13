export namespace main {
	
	export class Entry {
	    id: string;
	    name: string;
	    value: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.value = source["value"];
	        this.kind = source["kind"];
	    }
	}

}

