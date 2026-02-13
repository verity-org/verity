import { defineConfig } from 'astro/config';
import tailwind from '@astrojs/tailwind';

export default defineConfig({
  site: 'https://descope.github.io',
  base: '/',
  output: 'static',
  integrations: [tailwind()],
});
