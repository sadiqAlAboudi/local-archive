export namespace main {
	
	export class CreateDocumentInput {
	    doc_type: string;
	    serial_number: string;
	    issue_number: string;
	    doc_date: string;
	    department: string;
	    letter_number: string;
	    letter_date: string;
	    subject: string;
	    source_path: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateDocumentInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.doc_type = source["doc_type"];
	        this.serial_number = source["serial_number"];
	        this.issue_number = source["issue_number"];
	        this.doc_date = source["doc_date"];
	        this.department = source["department"];
	        this.letter_number = source["letter_number"];
	        this.letter_date = source["letter_date"];
	        this.subject = source["subject"];
	        this.source_path = source["source_path"];
	    }
	}
	export class UpdateDocumentInput {
	    id: number;
	    doc_type: string;
	    serial_number: string;
	    issue_number: string;
	    doc_date: string;
	    department: string;
	    letter_number: string;
	    letter_date: string;
	    subject: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateDocumentInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.doc_type = source["doc_type"];
	        this.serial_number = source["serial_number"];
	        this.issue_number = source["issue_number"];
	        this.doc_date = source["doc_date"];
	        this.department = source["department"];
	        this.letter_number = source["letter_number"];
	        this.letter_date = source["letter_date"];
	        this.subject = source["subject"];
	    }
	}

}

export namespace models {
	
	export class Document {
	    id: number;
	    doc_type: string;
	    serial_number: string;
	    issue_number: string;
	    doc_date: string;
	    department: string;
	    letter_number: string;
	    letter_date: string;
	    subject: string;
	    filename: string;
	    original_filename: string;
	    file_type: string;
	    mime_type: string;
	    file_size: number;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Document(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.doc_type = source["doc_type"];
	        this.serial_number = source["serial_number"];
	        this.issue_number = source["issue_number"];
	        this.doc_date = source["doc_date"];
	        this.department = source["department"];
	        this.letter_number = source["letter_number"];
	        this.letter_date = source["letter_date"];
	        this.subject = source["subject"];
	        this.filename = source["filename"];
	        this.original_filename = source["original_filename"];
	        this.file_type = source["file_type"];
	        this.mime_type = source["mime_type"];
	        this.file_size = source["file_size"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
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
	export class IndexViewData {
	    TotalDocs: number;
	    TotalIncoming: number;
	    TotalOutgoing: number;
	    StorageUsed: string;
	    Query: string;
	    FilterType: string;
	    HasFilter: boolean;
	    RestoreSuccess: boolean;
	    RestoreError: string;
	    ShowBackupReminder: boolean;
	    LastBackupDays: number;
	    Documents: Document[];
	
	    static createFrom(source: any = {}) {
	        return new IndexViewData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.TotalDocs = source["TotalDocs"];
	        this.TotalIncoming = source["TotalIncoming"];
	        this.TotalOutgoing = source["TotalOutgoing"];
	        this.StorageUsed = source["StorageUsed"];
	        this.Query = source["Query"];
	        this.FilterType = source["FilterType"];
	        this.HasFilter = source["HasFilter"];
	        this.RestoreSuccess = source["RestoreSuccess"];
	        this.RestoreError = source["RestoreError"];
	        this.ShowBackupReminder = source["ShowBackupReminder"];
	        this.LastBackupDays = source["LastBackupDays"];
	        this.Documents = this.convertValues(source["Documents"], Document);
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

export namespace updater {
	
	export class UpdateInfo {
	    available: boolean;
	    current_version: string;
	    latest_version: string;
	    release_title: string;
	    release_notes: string;
	    published_at: string;
	    download_url: string;
	    asset_size: number;
	    formatted_size: string;
	    asset_name: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.current_version = source["current_version"];
	        this.latest_version = source["latest_version"];
	        this.release_title = source["release_title"];
	        this.release_notes = source["release_notes"];
	        this.published_at = source["published_at"];
	        this.download_url = source["download_url"];
	        this.asset_size = source["asset_size"];
	        this.formatted_size = source["formatted_size"];
	        this.asset_name = source["asset_name"];
	    }
	}

}

