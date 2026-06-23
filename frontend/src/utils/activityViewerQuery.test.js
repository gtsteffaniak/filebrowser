import { describe, expect, it } from "vitest";

import {
  filterEventTypesForScope,
  formatEventTypeQueryValue,
  formatActivityViewerQueryString,
  encodeActivityViewerQueryValue,
  normalizeEventTypeQueryValue,
} from "./activityViewerQuery.js";

describe("normalizeEventTypeQueryValue", () => {
  it("parses comma-separated strings", () => {
    expect(normalizeEventTypeQueryValue("tokenCreate,tokenDelete")).toEqual([
      "tokenCreate",
      "tokenDelete",
    ]);
  });

  it("trims whitespace around comma-separated values", () => {
    expect(normalizeEventTypeQueryValue(" userCreate , userUpdate ")).toEqual([
      "userCreate",
      "userUpdate",
    ]);
  });

  it("accepts vue-router array query values", () => {
    expect(normalizeEventTypeQueryValue(["tokenCreate", "tokenDelete"])).toEqual([
      "tokenCreate",
      "tokenDelete",
    ]);
  });

  it("returns an empty list for blank values", () => {
    expect(normalizeEventTypeQueryValue("")).toEqual([]);
    expect(normalizeEventTypeQueryValue(null)).toEqual([]);
  });
});

describe("formatEventTypeQueryValue", () => {
  it("joins types with commas", () => {
    expect(formatEventTypeQueryValue(["tokenCreate", "tokenDelete"])).toBe(
      "tokenCreate,tokenDelete",
    );
  });

  it("returns an empty string when no types are selected", () => {
    expect(formatEventTypeQueryValue([])).toBe("");
  });
});

describe("filterEventTypesForScope", () => {
  it("keeps only types allowed for the current scope", () => {
    expect(
      filterEventTypesForScope(
        ["download", "tokenCreate"],
        ["download", "upload"],
      ),
    ).toEqual(["download"]);
  });
});

describe("formatActivityViewerQueryString", () => {
  it("keeps commas in eventType values readable", () => {
    expect(
      formatActivityViewerQueryString({
        eventType: "tokenCreate,tokenDelete",
      }),
    ).toBe("eventType=tokenCreate,tokenDelete");
  });

  it("still encodes unsafe characters in other params", () => {
    expect(
      formatActivityViewerQueryString({
        path: "/docs/my files",
      }),
    ).toBe("path=%2Fdocs%2Fmy%20files");
  });
});

describe("encodeActivityViewerQueryValue", () => {
  it("encodes each comma-separated event type segment", () => {
    expect(encodeActivityViewerQueryValue("eventType", "userCreate,userUpdate")).toBe(
      "userCreate,userUpdate",
    );
  });
});
