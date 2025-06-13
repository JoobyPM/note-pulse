import { defineConfig } from "vite";
import { viteStaticCopy } from "vite-plugin-static-copy";

export default defineConfig({
  root: "src",
  build: {
    outDir: "../dist",
    emptyOutDir: true,
  },
  resolve: {
    alias: { "@": "/src" },
  },
  plugins: [
    viteStaticCopy({
      targets: [
        {
          src: "../logo.svg",
          dest: "",
        },
      ],
    }),
  ],
});
