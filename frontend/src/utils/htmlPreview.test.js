import { describe, expect, it, vi } from 'vitest';

vi.mock('@/store', () => ({
  getters: { isShare: () => false },
  state: { shareInfo: {} },
}));

vi.mock('@/utils/url.js', () => ({
  getApiPath: (route, params) =>
    `/api/${route}?file=${encodeURIComponent(params.file)}&inline=true`,
  getPublicApiPath: (route, params) =>
    `/public/api/${route}?file=${encodeURIComponent(params.file[0])}&inline=true`,
  resolveRelativePath: (_base, href) => href.replace(/^\.\//, ''),
}));

import { buildPreviewResourceUrl } from './htmlPreview';

describe('htmlPreview resource URLs', () => {
  it('uses inline download for sibling assets, not media stream', () => {
    const url = buildPreviewResourceUrl('./style.css', '/docs/index.html', 'default');
    expect(url).toContain('/api/resources/download?');
    expect(url).toContain('inline=true');
    expect(url).not.toContain('/media/stream');
  });
});
