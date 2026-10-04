/**
 * Procedural lightning bolt paths for the Halloween login background.
 */

export type RandomFn = () => number;

interface Point {
  x: number;
  y: number;
}

interface SegmentChainOptions {
  start: Point;
  width: number;
  maxY: number;
  segments: number;
  jag: number;
  random: RandomFn;
  biasX?: number;
}

export interface LightningBoltOptions {
  width?: number;
  height?: number;
  segments?: number;
  /** Horizontal jitter per segment. */
  jag?: number;
  /** 0..1 chance per interior point. */
  branchChance?: number;
  /** Returns a value in [0, 1). */
  random?: RandomFn;
}

export interface LightningBolt {
  viewBox: string;
  main: string;
  branches: string[];
}

export interface LightningStrike extends LightningBolt {
  style: Record<string, string>;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function pointsToPath(points: Point[]): string {
  if (points.length === 0) return "";
  const [first, ...rest] = points;
  let d = `M ${first.x.toFixed(1)} ${first.y.toFixed(1)}`;
  for (const p of rest) {
    d += ` L ${p.x.toFixed(1)} ${p.y.toFixed(1)}`;
  }
  return d;
}

function growSegmentChain({
  start,
  width,
  maxY,
  segments,
  jag,
  random,
  biasX = 0,
}: SegmentChainOptions): Point[] {
  const points: Point[] = [start];
  let x = start.x;
  let y = start.y;
  const drop = Math.max(maxY - start.y, 1);
  const step = drop / segments;
  let momentum = biasX;

  for (let i = 0; i < segments; i++) {
    y = Math.min(y + step, maxY);
    momentum = momentum * 0.4 + (random() - 0.5) * jag;
    x = clamp(x + momentum, 6, width - 6);
    points.push({ x, y });
  }
  return points;
}

export function createLightningBolt({
  width = 90,
  height = 280,
  segments = 16,
  jag = 24,
  branchChance = 0.28,
  random = Math.random,
}: LightningBoltOptions = {}): LightningBolt {
  const mainPoints = growSegmentChain({
    start: { x: width / 2, y: 0 },
    width,
    maxY: height,
    segments,
    jag,
    random,
  });

  const branches: string[] = [];
  const branchStep = height / segments;
  for (const origin of mainPoints.slice(2, -2)) {
    if (random() > branchChance) continue;
    const dir = random() < 0.5 ? -1 : 1;
    const branchSegments = 3 + Math.floor(random() * 4);
    const branchDrop =
      branchStep * branchSegments * (0.85 + random() * 0.35);
    const branchPoints = growSegmentChain({
      start: origin,
      width,
      maxY: Math.min(height, origin.y + branchDrop),
      segments: branchSegments,
      jag: jag * 0.65,
      random,
      biasX: dir * jag * 0.35,
    });
    if (branchPoints.length > 1) {
      branches.push(pointsToPath(branchPoints));
    }
  }
  return {
    viewBox: `0 0 ${width} ${height}`,
    main: pointsToPath(mainPoints),
    branches,
  };
}

/** Random strike layout for the login viewport. */
export function createLightningStrike(
  random: RandomFn = Math.random,
): LightningStrike {
  const width = 70 + Math.floor(random() * 50);
  const height = 220 + Math.floor(random() * 120);
  const bolt = createLightningBolt({
    width,
    height,
    segments: 14 + Math.floor(random() * 6),
    jag: 18 + random() * 14,
    branchChance: 0.22 + random() * 0.2,
    random,
  });
  const leftPct = 8 + random() * 72;
  const topPct = 2 + random() * 10;
  const heightVh = 32 + random() * 28;
  const rotate = (random() - 0.5) * 14;
  return {
    ...bolt,
    style: {
      left: `${leftPct}%`,
      top: `${topPct}%`,
      height: `${heightVh}vh`,
      width: "auto",
      aspectRatio: `${width} / ${height}`,
      "--bolt-rotate": `${rotate.toFixed(1)}deg`,
      transformOrigin: "top center",
    },
  };
}

export function prefersReducedMotion(): boolean {
  if (typeof window === "undefined" || !window.matchMedia) return false;
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
