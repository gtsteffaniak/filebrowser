import { describe, expect, it } from "vitest";

import { buildActivityDetailBadges, buildActivityDetailRows, activityEventTypeBadgeClass } from "./activityDetails.js";

describe("access activity detail formatting", () => {
  const createRow = {
    eventType: "accessCreate",
    details: {
      changes: [
        { field: "ruleType", to: "deny" },
        { field: "ruleCategory", to: "user" },
        { field: "value", to: "admin" },
      ],
    },
  };

  const deleteRow = {
    eventType: "accessDelete",
    details: {
      changes: [
        { field: "ruleType", to: "deny" },
        { field: "ruleCategory", to: "user" },
        { field: "value", to: "admin" },
      ],
    },
  };

  const updateRow = {
    eventType: "accessUpdate",
    details: {
      changes: [{ field: "oldPath", from: "/a", to: "/b" }],
    },
  };

  it("shows create properties without from/to arrows", () => {
    const rows = buildActivityDetailRows(createRow, (key) => key);
    expect(rows.find((row) => row.id === "change-ruleType")?.value).toBe("deny");
    expect(rows.find((row) => row.id === "change-value")?.value).toBe("admin");
  });

  it("shows delete properties without from/to arrows", () => {
    const badges = buildActivityDetailBadges(deleteRow);
    expect(badges.find((badge) => badge.id === "change-ruleType")?.text).toBe(
      "ruleType: deny",
    );
  });

  it("shows update changes with from/to arrows", () => {
    const rows = buildActivityDetailRows(updateRow, (key) => key);
    expect(rows.find((row) => row.id === "change-oldPath")?.value).toBe("/a → /b");
  });
});

describe("activityEventTypeBadgeClass", () => {
  it("maps event categories to badge modifiers", () => {
    expect(activityEventTypeBadgeClass("accessDelete")).toBe("event-type-badge--delete");
    expect(activityEventTypeBadgeClass("accessCreate")).toBe("event-type-badge--create");
    expect(activityEventTypeBadgeClass("accessUpdate")).toBe("event-type-badge--change");
    expect(activityEventTypeBadgeClass("login")).toBe("event-type-badge--auth");
    expect(activityEventTypeBadgeClass("download")).toBe("event-type-badge--default");
  });
});
