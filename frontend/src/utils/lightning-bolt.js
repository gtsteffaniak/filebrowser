/**
 * Procedural lightning bolt paths for the Halloween login background.
 * @param {() => number} random - returns a value in [0, 1)
 */

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

function pointsToPath(points) {
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
}) {
  const points = [start];
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

/**
 * @param {object} [options]
 * @param {number} [options.width]
 * @param {number} [options.height]
 * @param {number} [options.segments]
 * @param {number} [options.jag] - horizontal jitter per segment
 * @param {number} [options.branchChance] - 0..1 chance per interior point
 * @param {() => number} [options.random]
 * @returns {{ viewBox: string, main: string, branches: string[] }}
 */
export function createLightningBolt({
  width = 90,
  height = 280,
  segments = 16,
  jag = 24,
  branchChance = 0.28,
  random = Math.random,
} = {}) {
  const mainPoints = growSegmentChain({
    start: { x: width / 2, y: 0 },
    width,
    maxY: height,
    segments,
    jag,
    random,
  });

  const branches = [];
  const branchStep = height / segments;
  for (let i = 2; i < mainPoints.length - 2; i++) {
    if (random() > branchChance) continue;
    const origin = mainPoints[i];
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

/**
 * Random strike layout for the login viewport.
 * @param {() => number} [random]
 */
export function createLightningStrike(random = Math.random) {
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

export function prefersReducedMotion() {
  if (typeof window === "undefined" || !window.matchMedia) return false;
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
