export namespace audio {
	
	export class PCMFrame {
	    // Go type: time
	    Timestamp: any;
	    Samples: number[];
	    SampleRate: number;
	    Channels: number;
	
	    static createFrom(source: any = {}) {
	        return new PCMFrame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Timestamp = this.convertValues(source["Timestamp"], null);
	        this.Samples = source["Samples"];
	        this.SampleRate = source["SampleRate"];
	        this.Channels = source["Channels"];
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

export namespace detector {
	
	export class Candidate {
	    ID: number;
	    FrequencyHz: number;
	    SNR: number;
	    // Go type: time
	    FirstSeen: any;
	    // Go type: time
	    LastSeen: any;
	    SyncScore: number;
	    Confidence: number;
	    State: number;
	
	    static createFrom(source: any = {}) {
	        return new Candidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.FrequencyHz = source["FrequencyHz"];
	        this.SNR = source["SNR"];
	        this.FirstSeen = this.convertValues(source["FirstSeen"], null);
	        this.LastSeen = this.convertValues(source["LastSeen"], null);
	        this.SyncScore = source["SyncScore"];
	        this.Confidence = source["Confidence"];
	        this.State = source["State"];
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

export namespace hotkey {
	
	export class Binding {
	    id: string;
	    name: string;
	    shortcut: string;
	    global: boolean;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Binding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.shortcut = source["shortcut"];
	        this.global = source["global"];
	        this.enabled = source["enabled"];
	    }
	}

}

export namespace logbook {
	
	export class Action {
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new Action(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	    }
	}
	export class Trigger {
	    event: string;
	    macro: string;
	
	    static createFrom(source: any = {}) {
	        return new Trigger(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.event = source["event"];
	        this.macro = source["macro"];
	    }
	}
	export class Rule {
	    id: string;
	    enabled: boolean;
	    trigger: Trigger;
	    actions: Action[];
	
	    static createFrom(source: any = {}) {
	        return new Rule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.enabled = source["enabled"];
	        this.trigger = this.convertValues(source["trigger"], Trigger);
	        this.actions = this.convertValues(source["actions"], Action);
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

export namespace model {
	
	export class AudioSettings {
	    inputDeviceId: string;
	    outputDeviceId: string;
	    sampleRate: number;
	    inputChannel: string;
	    outputChannel: string;
	    bufferMs: number;
	    txAudioLevel: number;
	    recordDirectory: string;
	
	    static createFrom(source: any = {}) {
	        return new AudioSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inputDeviceId = source["inputDeviceId"];
	        this.outputDeviceId = source["outputDeviceId"];
	        this.sampleRate = source["sampleRate"];
	        this.inputChannel = source["inputChannel"];
	        this.outputChannel = source["outputChannel"];
	        this.bufferMs = source["bufferMs"];
	        this.txAudioLevel = source["txAudioLevel"];
	        this.recordDirectory = source["recordDirectory"];
	    }
	}
	export class Callsign {
	    value: string;
	    start: number;
	    end: number;
	    confidence: number;
	
	    static createFrom(source: any = {}) {
	        return new Callsign(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.confidence = source["confidence"];
	    }
	}
	export class DecodeMessage {
	    sequence: number;
	    // Go type: time
	    signalUtc: any;
	    // Go type: time
	    receivedUtc: any;
	    snr: number;
	    dt: number;
	    frequencyHz: number;
	    message: string;
	    callsigns?: Callsign[];
	    confidence: number;
	    jttyTerminal?: string;
	    jttyMessageId?: number;
	
	    static createFrom(source: any = {}) {
	        return new DecodeMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sequence = source["sequence"];
	        this.signalUtc = this.convertValues(source["signalUtc"], null);
	        this.receivedUtc = this.convertValues(source["receivedUtc"], null);
	        this.snr = source["snr"];
	        this.dt = source["dt"];
	        this.frequencyHz = source["frequencyHz"];
	        this.message = source["message"];
	        this.callsigns = this.convertValues(source["callsigns"], Callsign);
	        this.confidence = source["confidence"];
	        this.jttyTerminal = source["jttyTerminal"];
	        this.jttyMessageId = source["jttyMessageId"];
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
	export class DecoderSettings {
	    threads: number;
	    mode: string;
	    thresholdDb: number;
	    frequencyToleranceHz: number;
	    trackerToleranceHz: number;
	    trackerTtlMs: number;
	
	    static createFrom(source: any = {}) {
	        return new DecoderSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.threads = source["threads"];
	        this.mode = source["mode"];
	        this.thresholdDb = source["thresholdDb"];
	        this.frequencyToleranceHz = source["frequencyToleranceHz"];
	        this.trackerToleranceHz = source["trackerToleranceHz"];
	        this.trackerTtlMs = source["trackerTtlMs"];
	    }
	}
	export class FrequencyBandSettings {
	    band: string;
	    frequencies: number[];
	    defaultHz: number;
	
	    static createFrom(source: any = {}) {
	        return new FrequencyBandSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.band = source["band"];
	        this.frequencies = source["frequencies"];
	        this.defaultHz = source["defaultHz"];
	    }
	}
	export class LayoutSettings {
	    waterfallHeight: number;
	    operationHeight: number;
	    windowWidth: number;
	    windowHeight: number;
	
	    static createFrom(source: any = {}) {
	        return new LayoutSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.waterfallHeight = source["waterfallHeight"];
	        this.operationHeight = source["operationHeight"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	    }
	}
	export class MacroConfig {
	    id: string;
	    name: string;
	    shortcut: string;
	    template: string;
	    enabled: boolean;
	    global: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MacroConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.shortcut = source["shortcut"];
	        this.template = source["template"];
	        this.enabled = source["enabled"];
	        this.global = source["global"];
	    }
	}
	export class RadioProfile {
	    rigName: string;
	    rigModelId: number;
	    rigctldPath: string;
	    host: string;
	    port: number;
	    serialPort: string;
	    baud: number;
	    dataBits: number;
	    stopBits: number;
	    handshake: string;
	    pttMethod: string;
	    pttSerialPort: string;
	    splitMode: string;
	    txAudioSource: string;
	    forceDtr: string;
	    forceRts: string;
	    pollIntervalSec: number;
	    mode: string;
	    passbandHz: number;
	    readPowerSWR: boolean;
	    haltOnSWR: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RadioProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rigName = source["rigName"];
	        this.rigModelId = source["rigModelId"];
	        this.rigctldPath = source["rigctldPath"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.serialPort = source["serialPort"];
	        this.baud = source["baud"];
	        this.dataBits = source["dataBits"];
	        this.stopBits = source["stopBits"];
	        this.handshake = source["handshake"];
	        this.pttMethod = source["pttMethod"];
	        this.pttSerialPort = source["pttSerialPort"];
	        this.splitMode = source["splitMode"];
	        this.txAudioSource = source["txAudioSource"];
	        this.forceDtr = source["forceDtr"];
	        this.forceRts = source["forceRts"];
	        this.pollIntervalSec = source["pollIntervalSec"];
	        this.mode = source["mode"];
	        this.passbandHz = source["passbandHz"];
	        this.readPowerSWR = source["readPowerSWR"];
	        this.haltOnSWR = source["haltOnSWR"];
	    }
	}
	export class RadioSettings {
	    configVersion: number;
	    local: RadioProfile;
	    net: RadioProfile;
	    backend: string;
	    enabled: boolean;
	    rigName: string;
	    rigModelId: number;
	    rigctldPath: string;
	    host: string;
	    port: number;
	    serialPort: string;
	    baud: number;
	    dataBits: number;
	    stopBits: number;
	    handshake: string;
	    pttMethod: string;
	    pttSerialPort: string;
	    splitMode: string;
	    txAudioSource: string;
	    forceDtr: string;
	    forceRts: string;
	    pollIntervalSec: number;
	    mode: string;
	    passbandHz: number;
	    readPowerSWR: boolean;
	    haltOnSWR: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RadioSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configVersion = source["configVersion"];
	        this.local = this.convertValues(source["local"], RadioProfile);
	        this.net = this.convertValues(source["net"], RadioProfile);
	        this.backend = source["backend"];
	        this.enabled = source["enabled"];
	        this.rigName = source["rigName"];
	        this.rigModelId = source["rigModelId"];
	        this.rigctldPath = source["rigctldPath"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.serialPort = source["serialPort"];
	        this.baud = source["baud"];
	        this.dataBits = source["dataBits"];
	        this.stopBits = source["stopBits"];
	        this.handshake = source["handshake"];
	        this.pttMethod = source["pttMethod"];
	        this.pttSerialPort = source["pttSerialPort"];
	        this.splitMode = source["splitMode"];
	        this.txAudioSource = source["txAudioSource"];
	        this.forceDtr = source["forceDtr"];
	        this.forceRts = source["forceRts"];
	        this.pollIntervalSec = source["pollIntervalSec"];
	        this.mode = source["mode"];
	        this.passbandHz = source["passbandHz"];
	        this.readPowerSWR = source["readPowerSWR"];
	        this.haltOnSWR = source["haltOnSWR"];
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
	export class WaterfallSettings {
	    minHz: number;
	    maxHz: number;
	    fft: number;
	    hop: number;
	
	    static createFrom(source: any = {}) {
	        return new WaterfallSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.minHz = source["minHz"];
	        this.maxHz = source["maxHz"];
	        this.fft = source["fft"];
	        this.hop = source["hop"];
	    }
	}
	export class Settings {
	    myCall: string;
	    myGrid: string;
	    audio: AudioSettings;
	    radio: RadioSettings;
	    waterfall: WaterfallSettings;
	    frequencies: FrequencyBandSettings[];
	    decoder: DecoderSettings;
	    layout: LayoutSettings;
	    macros: MacroConfig[];
	    automation: logbook.Rule[];
	    logbookPath: string;
	    decodeLogMode: string;
	    decodeLogEnabled: boolean;
	    recordEnabled: boolean;
	    decodeWindowLimit: number;
	    duplexMode: boolean;
	    autoFollowTail: boolean;
	    autoStartMonitor: boolean;
	    exchangeSerialNumber: number;
	    rxFrequencyHz: number;
	    txFrequencyHz: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.myCall = source["myCall"];
	        this.myGrid = source["myGrid"];
	        this.audio = this.convertValues(source["audio"], AudioSettings);
	        this.radio = this.convertValues(source["radio"], RadioSettings);
	        this.waterfall = this.convertValues(source["waterfall"], WaterfallSettings);
	        this.frequencies = this.convertValues(source["frequencies"], FrequencyBandSettings);
	        this.decoder = this.convertValues(source["decoder"], DecoderSettings);
	        this.layout = this.convertValues(source["layout"], LayoutSettings);
	        this.macros = this.convertValues(source["macros"], MacroConfig);
	        this.automation = this.convertValues(source["automation"], logbook.Rule);
	        this.logbookPath = source["logbookPath"];
	        this.decodeLogMode = source["decodeLogMode"];
	        this.decodeLogEnabled = source["decodeLogEnabled"];
	        this.recordEnabled = source["recordEnabled"];
	        this.decodeWindowLimit = source["decodeWindowLimit"];
	        this.duplexMode = source["duplexMode"];
	        this.autoFollowTail = source["autoFollowTail"];
	        this.autoStartMonitor = source["autoStartMonitor"];
	        this.exchangeSerialNumber = source["exchangeSerialNumber"];
	        this.rxFrequencyHz = source["rxFrequencyHz"];
	        this.txFrequencyHz = source["txFrequencyHz"];
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

export namespace waterfall {
	
	export class Frame {
	    // Go type: time
	    timestamp: any;
	    startHz: number;
	    binHz: number;
	    power: number[];
	
	    static createFrom(source: any = {}) {
	        return new Frame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.startHz = source["startHz"];
	        this.binHz = source["binHz"];
	        this.power = source["power"];
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

