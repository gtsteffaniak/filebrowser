import { describe, expect, it, vi } from "vitest";
import { commitSeek, type SeekablePlayer } from "@/player/ui/seekOnRelease.ts";

interface PlayerOptions {
  duration?: number;
  seekValue?: string;
  seekMax?: number;
}

function makePlayer({ duration = 200, seekValue = "50", seekMax = 100 }: PlayerOptions = {}) {
  const seek = document.createElement("input");
  seek.type = "range";
  seek.min = "0";
  seek.max = String(seekMax);
  seek.value = seekValue;
  const player: SeekablePlayer = {
    duration,
    currentTime: 0,
    elements: {
      inputs: { seek },
      progress: null,
    },
  };
  return { player, seek };
}

const changeEvent = (seek: HTMLInputElement): Event => {
  const event = new Event("change");
  Object.defineProperty(event, "currentTarget", { value: seek });
  return event;
};

describe("commitSeek onSeekTarget", () => {
  it("reports the intended target in seconds before writing currentTime", () => {
    const { player, seek } = makePlayer({ duration: 200, seekValue: "50" });
    const onSeekTarget = vi.fn();
    commitSeek(player, changeEvent(seek), onSeekTarget);
    expect(onSeekTarget).toHaveBeenCalledWith(100);
    expect(player.currentTime).toBe(100);
  });

  it("reports the true intent even when it lies beyond the playable range", () => {
    const { player, seek } = makePlayer({ duration: 3600, seekValue: "90" });
    const onSeekTarget = vi.fn();
    commitSeek(player, changeEvent(seek), onSeekTarget);
    expect(onSeekTarget).toHaveBeenCalledWith(3240);
    expect(player.currentTime).toBe(3240);
  });

  it("works without the callback", () => {
    const { player, seek } = makePlayer({ duration: 100, seekValue: "25" });
    expect(() => commitSeek(player, changeEvent(seek))).not.toThrow();
    expect(player.currentTime).toBe(25);
  });

  it("does nothing for a missing or zero duration", () => {
    const { player, seek } = makePlayer({ duration: 0 });
    const onSeekTarget = vi.fn();
    commitSeek(player, changeEvent(seek), onSeekTarget);
    expect(onSeekTarget).not.toHaveBeenCalled();
    expect(player.currentTime).toBe(0);
  });
});
