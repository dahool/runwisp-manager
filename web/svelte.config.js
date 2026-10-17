import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  kit: {
    adapter: adapter({ fallback: 'index.html' }),
    paths: { base: '/manager' },
    csp: {
      mode: 'hash',
      directives: {
        'script-src': ['self'],
        'style-src': ['self', 'unsafe-inline']
      }
    }
  }
};

export default config;
