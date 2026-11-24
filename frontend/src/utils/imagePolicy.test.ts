import DOMPurify from "dompurify";
import { describe, expect, it } from "vitest";
import { withImagePolicy } from "./imagePolicy.ts";

describe("withImagePolicy", () => {
  it("drops external SVG href and CSS url() styles while the callback runs", () => {
    const out = withImagePolicy(() =>
      DOMPurify.sanitize(
        '<svg><image href="https://example.com/a.png"></image></svg><div style="background:url(x.png)">a</div><div style="color:red">b</div>',
      ) as string,
    );
    expect(out).not.toContain("example.com");
    expect(out).not.toContain("url(");
    expect(out).toContain('style="color:red"');
  });

  it("does not change the shared DOMPurify instance outside the callback", () => {
    withImagePolicy(() => DOMPurify.sanitize("<p>warm</p>"));
    expect(DOMPurify.sanitize('<img src="https://example.com/a.png">')).toContain('src="https://example.com/a.png"');
  });
});
