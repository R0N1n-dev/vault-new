export namespace main {
	
	export class AppInfo {
	    name: string;
	    version: string;
	    platform: string;
	    vaultPath: string;
	    dataDir: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.platform = source["platform"];
	        this.vaultPath = source["vaultPath"];
	        this.dataDir = source["dataDir"];
	    }
	}
	export class AutoLockState {
	    enabled: boolean;
	    timeoutSeconds: number;
	    secondsRemaining: number;
	
	    static createFrom(source: any = {}) {
	        return new AutoLockState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.timeoutSeconds = source["timeoutSeconds"];
	        this.secondsRemaining = source["secondsRemaining"];
	    }
	}
	export class Status {
	    exists: boolean;
	    unlocked: boolean;
	    info?: vault.Info;
	    itemCount: number;
	    corrupt: boolean;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exists = source["exists"];
	        this.unlocked = source["unlocked"];
	        this.info = this.convertValues(source["info"], vault.Info);
	        this.itemCount = source["itemCount"];
	        this.corrupt = source["corrupt"];
	        this.message = source["message"];
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

}

export namespace vault {
	
	export class AuditFinding {
	    kind: string;
	    severity: string;
	    itemId: string;
	    title: string;
	    detail: string;
	    priority: number;
	
	    static createFrom(source: any = {}) {
	        return new AuditFinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.severity = source["severity"];
	        this.itemId = source["itemId"];
	        this.title = source["title"];
	        this.detail = source["detail"];
	        this.priority = source["priority"];
	    }
	}
	export class AuditReport {
	    score: number;
	    items: number;
	    affected: number;
	    weak: number;
	    reused: number;
	    stale: number;
	    noTotp: number;
	    cards: number;
	    findings: AuditFinding[];
	    trashed: number;
	    // Go type: time
	    checkedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new AuditReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.score = source["score"];
	        this.items = source["items"];
	        this.affected = source["affected"];
	        this.weak = source["weak"];
	        this.reused = source["reused"];
	        this.stale = source["stale"];
	        this.noTotp = source["noTotp"];
	        this.cards = source["cards"];
	        this.findings = this.convertValues(source["findings"], AuditFinding);
	        this.trashed = source["trashed"];
	        this.checkedAt = this.convertValues(source["checkedAt"], null);
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
	export class Card {
	    cardholder?: string;
	    number?: string;
	    brand?: string;
	    expMonth?: number;
	    expYear?: number;
	    cvc?: string;
	
	    static createFrom(source: any = {}) {
	        return new Card(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cardholder = source["cardholder"];
	        this.number = source["number"];
	        this.brand = source["brand"];
	        this.expMonth = source["expMonth"];
	        this.expYear = source["expYear"];
	        this.cvc = source["cvc"];
	    }
	}
	export class CustomField {
	    label: string;
	    value: string;
	    secret?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CustomField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.value = source["value"];
	        this.secret = source["secret"];
	    }
	}
	export class Strength {
	    score: number;
	    label: string;
	    entropyBits: number;
	    warning?: string;
	    suggestions?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Strength(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.score = source["score"];
	        this.label = source["label"];
	        this.entropyBits = source["entropyBits"];
	        this.warning = source["warning"];
	        this.suggestions = source["suggestions"];
	    }
	}
	export class Generated {
	    value: string;
	    strength: Strength;
	    entropyBits: number;
	
	    static createFrom(source: any = {}) {
	        return new Generated(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.strength = this.convertValues(source["strength"], Strength);
	        this.entropyBits = source["entropyBits"];
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
	export class GeneratorOptions {
	    mode: string;
	    length: number;
	    uppercase: boolean;
	    lowercase: boolean;
	    digits: boolean;
	    symbols: boolean;
	    excludeAmbiguous: boolean;
	    requireEverySelected: boolean;
	    words: number;
	    separator: string;
	    capitalize: boolean;
	    appendNumber: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GeneratorOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.length = source["length"];
	        this.uppercase = source["uppercase"];
	        this.lowercase = source["lowercase"];
	        this.digits = source["digits"];
	        this.symbols = source["symbols"];
	        this.excludeAmbiguous = source["excludeAmbiguous"];
	        this.requireEverySelected = source["requireEverySelected"];
	        this.words = source["words"];
	        this.separator = source["separator"];
	        this.capitalize = source["capitalize"];
	        this.appendNumber = source["appendNumber"];
	    }
	}
	export class HistoryEntry {
	    password: string;
	    // Go type: time
	    changedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.password = source["password"];
	        this.changedAt = this.convertValues(source["changedAt"], null);
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
	export class ImportOptions {
	    dryRun: boolean;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dryRun = source["dryRun"];
	        this.mode = source["mode"];
	    }
	}
	export class ImportRow {
	    line: number;
	    title: string;
	    username?: string;
	    url?: string;
	    type: string;
	    status: string;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.title = source["title"];
	        this.username = source["username"];
	        this.url = source["url"];
	        this.type = source["type"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	    }
	}
	export class ImportResult {
	    format: string;
	    dryRun: boolean;
	    mode: string;
	    total: number;
	    new: number;
	    duplicates: number;
	    invalid: number;
	    unsupported: number;
	    imported: number;
	    rows: ImportRow[];
	    warnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.dryRun = source["dryRun"];
	        this.mode = source["mode"];
	        this.total = source["total"];
	        this.new = source["new"];
	        this.duplicates = source["duplicates"];
	        this.invalid = source["invalid"];
	        this.unsupported = source["unsupported"];
	        this.imported = source["imported"];
	        this.rows = this.convertValues(source["rows"], ImportRow);
	        this.warnings = source["warnings"];
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
	
	export class Info {
	    path: string;
	    vaultId: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    revision: number;
	    hint?: string;
	    format: number;
	    kdf: string;
	    memoryMiB: number;
	    passes: number;
	    parallelism: number;
	    backupAvailable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.vaultId = source["vaultId"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.revision = source["revision"];
	        this.hint = source["hint"];
	        this.format = source["format"];
	        this.kdf = source["kdf"];
	        this.memoryMiB = source["memoryMiB"];
	        this.passes = source["passes"];
	        this.parallelism = source["parallelism"];
	        this.backupAvailable = source["backupAvailable"];
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
	export class Item {
	    id: string;
	    type: string;
	    title: string;
	    username?: string;
	    password?: string;
	    url?: string;
	    notes?: string;
	    tags?: string[];
	    favorite?: boolean;
	    totp?: string;
	    card?: Card;
	    custom?: CustomField[];
	    history?: HistoryEntry[];
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    // Go type: time
	    deletedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.title = source["title"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.url = source["url"];
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	        this.favorite = source["favorite"];
	        this.totp = source["totp"];
	        this.card = this.convertValues(source["card"], Card);
	        this.custom = this.convertValues(source["custom"], CustomField);
	        this.history = this.convertValues(source["history"], HistoryEntry);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.deletedAt = this.convertValues(source["deletedAt"], null);
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
	export class ListOptions {
	    query: string;
	    type: string;
	    tag: string;
	    favoritesOnly: boolean;
	    sort: string;
	
	    static createFrom(source: any = {}) {
	        return new ListOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.type = source["type"];
	        this.tag = source["tag"];
	        this.favoritesOnly = source["favoritesOnly"];
	        this.sort = source["sort"];
	    }
	}
	export class Settings {
	    autoLockMinutes: number;
	    clipboardClearSeconds: number;
	    lockOnBlur: boolean;
	    theme: string;
	    generator: GeneratorOptions;
	    passwordHistoryMax: number;
	    trashRetentionDays: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.autoLockMinutes = source["autoLockMinutes"];
	        this.clipboardClearSeconds = source["clipboardClearSeconds"];
	        this.lockOnBlur = source["lockOnBlur"];
	        this.theme = source["theme"];
	        this.generator = this.convertValues(source["generator"], GeneratorOptions);
	        this.passwordHistoryMax = source["passwordHistoryMax"];
	        this.trashRetentionDays = source["trashRetentionDays"];
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
	export class Stats {
	    total: number;
	    logins: number;
	    notes: number;
	    cards: number;
	    favorites: number;
	    withTOTP: number;
	    trashed: number;
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.logins = source["logins"];
	        this.notes = source["notes"];
	        this.cards = source["cards"];
	        this.favorites = source["favorites"];
	        this.withTOTP = source["withTOTP"];
	        this.trashed = source["trashed"];
	    }
	}
	
	export class Summary {
	    id: string;
	    type: string;
	    title: string;
	    username?: string;
	    url?: string;
	    tags?: string[];
	    favorite: boolean;
	    hasPassword: boolean;
	    hasTOTP: boolean;
	    hasNotes: boolean;
	    // Go type: time
	    updatedAt: any;
	    // Go type: time
	    deletedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.title = source["title"];
	        this.username = source["username"];
	        this.url = source["url"];
	        this.tags = source["tags"];
	        this.favorite = source["favorite"];
	        this.hasPassword = source["hasPassword"];
	        this.hasTOTP = source["hasTOTP"];
	        this.hasNotes = source["hasNotes"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.deletedAt = this.convertValues(source["deletedAt"], null);
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
	export class TOTPResult {
	    code: string;
	    digits: number;
	    period: number;
	    remaining: number;
	
	    static createFrom(source: any = {}) {
	        return new TOTPResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.digits = source["digits"];
	        this.period = source["period"];
	        this.remaining = source["remaining"];
	    }
	}
	export class TagCount {
	    name: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new TagCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.count = source["count"];
	    }
	}

}

