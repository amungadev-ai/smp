<script lang="ts">
 import Icon from '$lib/components/Icon.svelte';import type {Project} from '$lib/model';
 import {projectProcess} from '$lib/stores/workspace';import {infraState,infraCommand} from '$lib/infra/store';import {runtimeProfiles} from '$lib/infra/model.js';
 export let project:Project;
 let error='',notice='',profile='';
 $: configured=$infraState.runtimeAssignments[project.id]||'';
 $: if(profile!==configured)profile=configured;
 function process(operation:'Start'|'Stop'|'Restart'){try{projectProcess(project.id,operation);notice=operation+' simulated; no real process was changed.';error='';}catch(e){error=e instanceof Error?e.message:'Process control unavailable.';}}
 function setProfile(){try{infraCommand({type:'runtime-assign',project:project.id,profile});error='';notice='Runtime preference saved, no runtime installed.';}catch(e){error=e instanceof Error?e.message:'Profile unavailable.';}}
 function setAuto(value:boolean){try{infraCommand({type:'auto-deploy',project:project.id,enabled:value});error='';notice='Auto-deploy preference saved. No webhook configured.';}catch(e){error=e instanceof Error?e.message:'Setting unavailable.';}}
</script>
<section class="card stack"><div><h3>Application controls</h3><p class="subtitle">Manage example process state and runtime preferences; no actual VPS changes occur.</p></div>
<div class="row wrap">{#if project.status==='Live'}<button class="btn secondary" on:click={()=>process('Restart')}><Icon name="Monitoring"/> Simulate restart</button><button class="btn secondary" on:click={()=>process('Stop')}>Simulate stop</button>{:else if project.status==='Stopped'}<button class="btn secondary" on:click={()=>process('Start')}>Simulate start</button>{:else}<a href={'/projects/'+project.id+'/deployments'} class="btn secondary">Deployment needed</a>{/if}</div>
<form class="native-inline" on:submit|preventDefault={setProfile}><label class="field grow">Runtime profile<select bind:value={profile}><option value="">No preference</option>{#each runtimeProfiles as p}<option value={p.id}>{p.name}</option>{/each}</select></label><button class="btn secondary" type="submit" style="align-self:end">Save preference</button></form>
{#if project.source==='GitHub'}<div class="setting-row"><div><strong>Auto-deploy preference</strong><small style="display:block">No GitHub webhook or deployment trigger is configured.</small></div><label class="check"><input type="checkbox" checked={$infraState.autoDeploy[project.id]||false} on:change={(e)=>setAuto(e.currentTarget.checked)}/> {$infraState.autoDeploy[project.id]?'On (demo)':'Off'}</label></div>{/if}
{#if error}<p class="error" role="alert">{error}</p>{/if}{#if notice}<p class="subtitle" role="status">{notice}</p>{/if}</section>
