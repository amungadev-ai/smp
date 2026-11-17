<script lang="ts">import Icon from './Icon.svelte';export let framework:string;</script>
{#if /wordpress/i.test(framework)}
<svg viewBox="0 0 40 40" aria-hidden="true"><circle cx="20" cy="20" r="18" fill="#21759b"/><text x="20" y="28" text-anchor="middle" fill="white" font-size="25" font-family="Georgia,serif">W</text></svg>
{:else if /next\.js/i.test(framework)}
<svg viewBox="0 0 40 40" aria-hidden="true"><circle cx="20" cy="20" r="18" fill="#111827"/><path d="M13 28V12l19 23M27 12v13" stroke="white" stroke-width="2.4" stroke-linecap="round" fill="none"/></svg>
{:else if /node|express/i.test(framework)}
<svg viewBox="0 0 40 40" aria-hidden="true"><path d="M20 2 36 11v18L20 38 4 29V11Z" fill="#ecf7ed" stroke="#43853d" stroke-width="2"/><text x="20" y="25" fill="#43853d" text-anchor="middle" font-size="15" font-weight="700">JS</text></svg>
{:else}<Icon name={/docker/i.test(framework)?"Containers":"Code"}/>{/if}
