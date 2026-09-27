export namespace models {
	
	export class AgentStep {
	    index: number;
	    tool: string;
	    args: string;
	    result: string;
	    error: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new AgentStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.tool = source["tool"];
	        this.args = source["args"];
	        this.result = source["result"];
	        this.error = source["error"];
	        this.durationMs = source["durationMs"];
	    }
	}
	export class AnalysisRecord {
	    id: number;
	    coinId: string;
	    symbol: string;
	    name: string;
	    currency: string;
	    model: string;
	    direction: string;
	    confidence: number;
	    support: number;
	    resistance: number;
	    summary: string;
	    priceAtAnalysis: number;
	    steps: AgentStep[];
	    promptTokens: number;
	    completionTokens: number;
	    cached: boolean;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    evaluatedAt?: any;
	    priceAfter24h: number;
	    returnPct: number;
	    correct?: boolean;
	    evalAttempts: number;
	    evalError: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalysisRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.coinId = source["coinId"];
	        this.symbol = source["symbol"];
	        this.name = source["name"];
	        this.currency = source["currency"];
	        this.model = source["model"];
	        this.direction = source["direction"];
	        this.confidence = source["confidence"];
	        this.support = source["support"];
	        this.resistance = source["resistance"];
	        this.summary = source["summary"];
	        this.priceAtAnalysis = source["priceAtAnalysis"];
	        this.steps = this.convertValues(source["steps"], AgentStep);
	        this.promptTokens = source["promptTokens"];
	        this.completionTokens = source["completionTokens"];
	        this.cached = source["cached"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.evaluatedAt = this.convertValues(source["evaluatedAt"], null);
	        this.priceAfter24h = source["priceAfter24h"];
	        this.returnPct = source["returnPct"];
	        this.correct = source["correct"];
	        this.evalAttempts = source["evalAttempts"];
	        this.evalError = source["evalError"];
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
	export class CachedPrice {
	    id: number;
	    coinId: string;
	    symbol: string;
	    name: string;
	    currency: string;
	    price: number;
	    change24h: number;
	    volume24h: number;
	    marketCap: number;
	    high24h: number;
	    low24h: number;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new CachedPrice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.coinId = source["coinId"];
	        this.symbol = source["symbol"];
	        this.name = source["name"];
	        this.currency = source["currency"];
	        this.price = source["price"];
	        this.change24h = source["change24h"];
	        this.volume24h = source["volume24h"];
	        this.marketCap = source["marketCap"];
	        this.high24h = source["high24h"];
	        this.low24h = source["low24h"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class CoinSearchResult {
	    id: string;
	    symbol: string;
	    name: string;
	    thumb: string;
	
	    static createFrom(source: any = {}) {
	        return new CoinSearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.symbol = source["symbol"];
	        this.name = source["name"];
	        this.thumb = source["thumb"];
	    }
	}
	export class DirectionStats {
	    direction: string;
	    evaluated: number;
	    correct: number;
	    accuracy: number;
	
	    static createFrom(source: any = {}) {
	        return new DirectionStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.direction = source["direction"];
	        this.evaluated = source["evaluated"];
	        this.correct = source["correct"];
	        this.accuracy = source["accuracy"];
	    }
	}
	export class EvalStats {
	    total: number;
	    evaluated: number;
	    pending: number;
	    unscorable: number;
	    correct: number;
	    accuracy: number;
	    avgConfidenceCorrect: number;
	    avgConfidenceWrong: number;
	    byDirection: DirectionStats[];
	    neutralBandPct: number;
	
	    static createFrom(source: any = {}) {
	        return new EvalStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.evaluated = source["evaluated"];
	        this.pending = source["pending"];
	        this.unscorable = source["unscorable"];
	        this.correct = source["correct"];
	        this.accuracy = source["accuracy"];
	        this.avgConfidenceCorrect = source["avgConfidenceCorrect"];
	        this.avgConfidenceWrong = source["avgConfidenceWrong"];
	        this.byDirection = this.convertValues(source["byDirection"], DirectionStats);
	        this.neutralBandPct = source["neutralBandPct"];
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
	export class MarketOverview {
	    currency: string;
	    totalMarketCap: number;
	    totalVolume24h: number;
	    btcDominance: number;
	    ethDominance: number;
	    activeCoins: number;
	    marketCapChange24h: number;
	
	    static createFrom(source: any = {}) {
	        return new MarketOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currency = source["currency"];
	        this.totalMarketCap = source["totalMarketCap"];
	        this.totalVolume24h = source["totalVolume24h"];
	        this.btcDominance = source["btcDominance"];
	        this.ethDominance = source["ethDominance"];
	        this.activeCoins = source["activeCoins"];
	        this.marketCapChange24h = source["marketCapChange24h"];
	    }
	}
	export class NewsItem {
	    title: string;
	    url: string;
	    source: string;
	    publishedAt: string;
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new NewsItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.url = source["url"];
	        this.source = source["source"];
	        this.publishedAt = source["publishedAt"];
	        this.summary = source["summary"];
	    }
	}
	export class PriceAlert {
	    id: number;
	    coinId: string;
	    symbol: string;
	    currency: string;
	    highPrice: number;
	    lowPrice: number;
	    enabled: boolean;
	    // Go type: time
	    triggeredAt?: any;
	    lastMessage: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PriceAlert(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.coinId = source["coinId"];
	        this.symbol = source["symbol"];
	        this.currency = source["currency"];
	        this.highPrice = source["highPrice"];
	        this.lowPrice = source["lowPrice"];
	        this.enabled = source["enabled"];
	        this.triggeredAt = this.convertValues(source["triggeredAt"], null);
	        this.lastMessage = source["lastMessage"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class Settings {
	    id: number;
	    openAiBase: string;
	    openAiModel: string;
	    currency: string;
	    refreshSecs: number;
	    apiKey: string;
	    hasApiKey: boolean;
	    apiKeyHint: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.openAiBase = source["openAiBase"];
	        this.openAiModel = source["openAiModel"];
	        this.currency = source["currency"];
	        this.refreshSecs = source["refreshSecs"];
	        this.apiKey = source["apiKey"];
	        this.hasApiKey = source["hasApiKey"];
	        this.apiKeyHint = source["apiKeyHint"];
	    }
	}
	export class WatchlistItem {
	    id: number;
	    coinId: string;
	    symbol: string;
	    name: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new WatchlistItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.coinId = source["coinId"];
	        this.symbol = source["symbol"];
	        this.name = source["name"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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

