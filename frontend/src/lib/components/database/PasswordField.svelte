<script lang="ts">
 import {samplePassword} from '$lib/database/store';
 export let value='';export let inputId='database-password';
 let visible=false;
</script>
<div class="db13-password"><label class="db13-field-title" for={inputId}>Password</label><div class="db13-password-row"><input id={inputId} class="input" type={visible?'text':'password'} value={value} on:input={(e)=>value=e.currentTarget.value} autocomplete="new-password" minlength="8" maxlength="128" required placeholder="Enter a sample password"/><button type="button" class="btn secondary" aria-label={visible?'Hide password':'Show password'} on:click={()=>visible=!visible}>{visible?'Hide':'Show'}</button><button type="button" class="btn secondary" on:click={()=>{value=samplePassword();visible=true}}>Generate</button></div><small>At least 8 characters. Use sample credentials only.</small></div>
