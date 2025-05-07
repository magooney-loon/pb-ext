The easiest way to start building a SvelteKit app

npx sv create frontend


make sure you choose the static-adapter and use the scripts in cmd/scripts/main.go

dont forget to make a root +layout.ts and add this
export const prerender = true;
export const trailingSlash = 'always';