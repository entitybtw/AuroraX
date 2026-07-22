// Tailwind 4 is wired through @tailwindcss/vite, not PostCSS. We declare an
// empty PostCSS config so PostCSS does not auto-discover the Tailwind v3
// install that lives in the parent monorepo's node_modules and try to
// process this workspace's @theme/@layer-using CSS with the v3 plugin.
export default {
  plugins: {},
};
