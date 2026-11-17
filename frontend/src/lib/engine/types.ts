/** Wire types matching the uploaded Go engine's /api/v1 contract. */
export type RuntimeState={installed:boolean;running:boolean;version:string};
export type Health={ok:boolean;version:string;mock_mode:boolean};
export type ServerInfo={hostname:string;os:string;kernel:string;arch:string;cpu_cores:number;mem_total:number;disk_total:number;disk_used:number;ip:string;uptime_s:number;docker:RuntimeState;caddy:RuntimeState;engine_version:string;mock_mode:boolean};
export type ReadinessCheck={id:string;label:string;ok:boolean;detail:string};
export type EngineProject={id:string;name:string;image:string;port:number;domain?:string;https?:boolean;status:'idle'|'deploying'|'running'|'stopped'|'failed';created:string;current_release?:string;release_count:number};
export type EngineRelease={id:string;project_id:string;image:string;tag:string;created:string;status:'queued'|'deploying'|'healthy'|'unhealthy'|'failed'|'rolled_back';health_ok?:boolean;health_latency_ms?:number};
export type JobLog={ts:string;message:string};
export type EngineJob={id:string;project_id:string;kind:'deploy'|'rollback'|'restart';status:'queued'|'running'|'succeeded'|'failed';created:string;started?:string;finished?:string;error?:string;release_id?:string;log:JobLog[]};
export type EngineLog={ts?:string;stream:'stdout'|'stderr'|'engine';message:string};
export type EngineDomain={id:string;name:string;project_id?:string;scheme:'http'|'https';status:'active'|'pending';created:string};
