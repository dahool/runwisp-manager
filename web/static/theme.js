(() => {
	const theme = localStorage.getItem('runwisp:theme') || 'auto';
	const dark = theme === 'dark' || (theme === 'auto' && matchMedia('(prefers-color-scheme: dark)').matches);
	document.documentElement.classList.toggle('dark', dark);
})();
