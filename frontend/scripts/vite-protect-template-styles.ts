// Vite plugin to keep the go template <style> on Vite CSS

import type { Plugin } from "vite";

const STYLE_BLOCK = /<style\b[^>]*>[\s\S]*?<\/style>/g;

export default function protectTemplateStyles(): Plugin[] {
  let blocks: string[] = [];
  const marker = (i: number) => `<!--go-template-style-${i}-->`;
  return [
    {
      name: "protect-template-styles:pre",
      apply: "build",
      transformIndexHtml: {
        order: "pre",
        handler(html: string) {
          blocks = [];
          return html.replace(STYLE_BLOCK, (block) => {
            if (!block.includes("{{")) return block;
            blocks.push(block);
            return marker(blocks.length - 1);
          });
        },
      },
    },
    {
      name: "protect-template-styles:post",
      apply: "build",
      transformIndexHtml: {
        order: "post",
        handler(html: string) {
          return blocks.reduce((out, block, i) => out.replace(marker(i), () => block), html);
        },
      },
    },
  ];
}
