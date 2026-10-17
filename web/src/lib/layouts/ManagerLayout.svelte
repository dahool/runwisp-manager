<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	import { Menu, ExternalLink } from '@lucide/svelte';
	import { Logo, ThemeToggle } from '@runwisp/ui';
	import { page } from '$app/state';
	let { children } = $props();

	const groups = [
		{ label: 'RunWisp', external: true, items: [{ label: 'Dashboard', href: '/' }, { label: 'Runs', href: '/runs' }, { label: 'Notifications', href: '/notifications' }] },
		{ label: 'Manage', items: [{ label: 'Tasks', href: '/manager/tasks' }, { label: 'Settings', href: '/manager/settings' }, { label: 'Git', href: '/manager/git' }, { label: 'Audit', href: '/manager/audit' }] }
	];
	let mobileOpen = $state(false);
</script>

<svelte:head><title>RunWisp Manager</title></svelte:head>

<div class="shell">
	<header class="topbar">
		<button class="menu" aria-label="Toggle navigation" onclick={() => mobileOpen = !mobileOpen}><Menu size={18} /></button>
		<a class="brand" href="/manager/" aria-label="RunWisp Manager home"><Logo size="sm" /><span>RunWisp <strong>Manager</strong></span></a>
		<div class="top-actions"><ThemeToggle /></div>
	</header>
	<button class:open={mobileOpen} class="backdrop" aria-label="Close navigation" onclick={() => mobileOpen = false}></button>
	<aside class:open={mobileOpen} class="sidebar">
		<nav aria-label="Primary navigation">
			{#each groups as group}
				<div class="nav-group">
					<div class="nav-label">{group.label}{#if group.external}<ExternalLink size={12} />{/if}</div>
					{#each group.items as item}
						<a class:active={page.url.pathname === item.href} href={item.href} onclick={() => mobileOpen = false}>{item.label}</a>
					{/each}
				</div>
			{/each}
		</nav>
		<div class="sidebar-foot">Manager shell <span>Phase 01</span></div>
	</aside>
	<main class="content">{@render children()}</main>
</div>

<style>
	:global(body) { font-family: var(--font-mono, ui-monospace, monospace); color: var(--color-foreground, #1e293b); background: var(--color-background, #fff); }
	.shell { min-height: 100%; background: var(--color-background, #fff); }
	.topbar { height: 56px; display: flex; align-items: center; border-bottom: 1px solid var(--color-border, #d8dee8); padding: 0 20px; gap: 16px; }
	.brand { display: flex; align-items: center; gap: 10px; color: inherit; text-decoration: none; font-size: 13px; }
	.brand strong { font-weight: 500; opacity: .62; }
	.menu, .theme { border: 0; background: transparent; color: inherit; cursor: pointer; padding: 7px; font: inherit; }
	.menu { display: none; }
	.top-actions { margin-left: auto; }
	.sidebar { position: fixed; top: 56px; bottom: 0; width: 232px; border-right: 1px solid var(--color-border, #d8dee8); padding: 24px 16px; display: flex; flex-direction: column; }
	.nav-group + .nav-group { margin-top: 28px; }
	.nav-label { display: flex; align-items: center; gap: 6px; font-size: 10px; margin: 0 8px 8px; opacity: .55; }
	.nav-group a { display: block; color: inherit; text-decoration: none; font-size: 12px; padding: 8px; border-radius: 3px; opacity: .72; }
	.nav-group a:hover, .nav-group a:focus-visible, .nav-group a.active { background: color-mix(in srgb, currentColor 8%, transparent); opacity: 1; outline: none; }
	.sidebar-foot { margin-top: auto; font-size: 10px; opacity: .5; padding: 8px; }
	.sidebar-foot span { float: right; }
	.content { margin-left: 232px; min-height: calc(100vh - 56px); padding: 32px; }
	.backdrop { display: none; border: 0; padding: 0; }
	@media (max-width: 700px) { .menu { display: block; } .sidebar { transform: translateX(-100%); transition: transform .15s ease; z-index: 2; background: var(--color-background, #fff); } .sidebar.open { transform: translateX(0); } .backdrop.open { display: block; position: fixed; inset: 56px 0 0; background: #0005; z-index: 1; } .content { margin-left: 0; padding: 24px 18px; } }
	@media (prefers-reduced-motion: reduce) { .sidebar { transition: none; } }
</style>
