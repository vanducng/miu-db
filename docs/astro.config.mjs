// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import react from '@astrojs/react';
import starlightLlmsTxt from 'starlight-llms-txt';
import remarkGfm from 'remark-gfm';

export default defineConfig({
  site: 'https://db.miu.sh',
  // GFM (tables, strikethrough) for MDX — .mdx does not get it by default.
  // NOTE: markdown.remarkPlugins is deprecated in Astro 6; migrate when bumping major.
  markdown: { remarkPlugins: [remarkGfm] },
  redirects: {
    '/go-install/': '/install/',
    '/go-daily-driver/': '/onboarding/',
    '/agent-cli/': '/cli-contract/',
    '/cloud-adapters/': '/authentication/',
    '/system-architecture/': '/architecture/',
    '/golang-architecture/': '/architecture/',
    '/tech-stack/': '/architecture/',
    '/deployment/': '/development/',
    '/development-guidelines/': '/development/',
  },
  integrations: [
    starlight({
      title: 'miu-db',
      logo: { src: './src/assets/logo.svg' },
      // Apply Starlight's markdown pipeline (asides, heading links) to the custom-loader content/ dir.
      markdown: { processedDirs: ['./content'] },
      description: 'Headless database CLI for humans and agents.',
      customCss: ['./src/styles/theme.css'],
      expressiveCode: {
        themes: ['catppuccin-mocha', 'catppuccin-latte'],
        styleOverrides: { borderRadius: '0.5rem' },
      },
      components: {
        ThemeSelect: './src/components/ThemeSelect.astro',
        SocialIcons: './src/components/SocialIcons.astro',
        Search: './src/components/Search.astro',
      },
      plugins: [
        starlightLlmsTxt({
          projectName: 'miu-db',
          description: 'Headless database CLI for humans and agents.',
        }),
      ],
      lastUpdated: true,
      sidebar: [
        { label: 'Getting started', link: '/onboarding/' },
        { label: 'Introduction', link: '/' },
        { label: 'Guides', items: ['install', 'usage', 'authentication', 'erd', 'activity'] },
        { label: 'Integration', items: ['mcp', 'cli-contract'] },
        { label: 'Internals', items: ['architecture', 'development'] },
        {
          label: 'Related docs',
          items: [
            { label: 'miu-cr', link: 'https://cr.miu.sh', attrs: { target: '_blank' } },
            { label: 'dotfiles', link: 'https://dotfiles.vanducng.dev', attrs: { target: '_blank' } },
            { label: 'skills', link: 'https://skills.vanducng.dev', attrs: { target: '_blank' } },
            { label: 'vd-cli', link: 'https://vd-cli.vanducng.dev', attrs: { target: '_blank' } },
          ],
        },
      ],
    }),
    react(),
  ],
});
