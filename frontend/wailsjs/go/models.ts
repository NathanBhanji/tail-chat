export namespace chat {
	
	export class FileInfo {
	    Filename: string;
	    Size: number;
	    State: number;
	    Error: string;
	    Path: string;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Filename = source["Filename"];
	        this.Size = source["Size"];
	        this.State = source["State"];
	        this.Error = source["Error"];
	        this.Path = source["Path"];
	    }
	}
	export class Group {
	    ID: string;
	    Name: string;
	    Members: string[];
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Members = source["Members"];
	    }
	}
	export class Reaction {
	    Emoji: string;
	    Sender: string;
	
	    static createFrom(source: any = {}) {
	        return new Reaction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Emoji = source["Emoji"];
	        this.Sender = source["Sender"];
	    }
	}
	export class Message {
	    ID: string;
	    Sender: string;
	    Content: string;
	    // Go type: time
	    Timestamp: any;
	    IsOwn: boolean;
	    GroupID: string;
	    State: number;
	    Reactions: Reaction[];
	    FileInfo?: FileInfo;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Sender = source["Sender"];
	        this.Content = source["Content"];
	        this.Timestamp = this.convertValues(source["Timestamp"], null);
	        this.IsOwn = source["IsOwn"];
	        this.GroupID = source["GroupID"];
	        this.State = source["State"];
	        this.Reactions = this.convertValues(source["Reactions"], Reaction);
	        this.FileInfo = this.convertValues(source["FileInfo"], FileInfo);
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

export namespace config {
	
	export class ThemeInfo {
	    name: string;
	    description: string;
	    author: string;
	    path: string;
	    isDefault: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ThemeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.author = source["author"];
	        this.path = source["path"];
	        this.isDefault = source["isDefault"];
	    }
	}

}

export namespace discovery {
	
	export class Peer {
	    Hostname: string;
	    DNSName: string;
	    TailscaleIP: string;
	    Online: boolean;
	    OS: string;
	    IsSelf: boolean;
	    RunningTailchat: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Peer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Hostname = source["Hostname"];
	        this.DNSName = source["DNSName"];
	        this.TailscaleIP = source["TailscaleIP"];
	        this.Online = source["Online"];
	        this.OS = source["OS"];
	        this.IsSelf = source["IsSelf"];
	        this.RunningTailchat = source["RunningTailchat"];
	    }
	}

}

export namespace tenor {
	
	export class MediaObject {
	    URL: string;
	    Dims: number[];
	    Size: number;
	
	    static createFrom(source: any = {}) {
	        return new MediaObject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.URL = source["URL"];
	        this.Dims = source["Dims"];
	        this.Size = source["Size"];
	    }
	}
	export class MediaFormats {
	    GIF: MediaObject;
	    TinyGIF: MediaObject;
	    NanoGIF: MediaObject;
	
	    static createFrom(source: any = {}) {
	        return new MediaFormats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.GIF = this.convertValues(source["GIF"], MediaObject);
	        this.TinyGIF = this.convertValues(source["TinyGIF"], MediaObject);
	        this.NanoGIF = this.convertValues(source["NanoGIF"], MediaObject);
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
	export class GIF {
	    ID: string;
	    Title: string;
	    URL: string;
	    Media: MediaFormats;
	
	    static createFrom(source: any = {}) {
	        return new GIF(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Title = source["Title"];
	        this.URL = source["URL"];
	        this.Media = this.convertValues(source["Media"], MediaFormats);
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

