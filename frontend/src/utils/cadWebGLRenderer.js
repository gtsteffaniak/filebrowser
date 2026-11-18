// cadWebGLRenderer.js — the OpenSW example's 3D view, ported from
// tools/webui.py: plain WebGL, no library. The view imitates SolidWorks
// (orthographic projection, shaded with black edges, gradient background,
// hovered face outlined in orange, selected face in blue, triad X red Y
// green Z blue, 15° rotation steps, zoom towards the cursor). Each vertex
// carries its face index; picking renders the scene offscreen with each
// face painted with its index and reads the pixel under the mouse.



const SW = {
  defaut: [0.894, 0.894, 0.894], // face with no colour in the file
  survol: [1.0, 0.502, 0.0], // Dynamic Highlight, 0x0080FF in BGR
  choisie: [0.0, 0.502, 1.0], // Selected Face, Shaded, 0xFF8000 in BGR
  arete: [0.0, 0.0, 0.0], // HLR Edges in Shaded Mode
  pas: Math.PI / 12, // View Rotate Increment: 15°
  axes: ["#ff0000", "#008000", "#0000ff"], // Reference Triad X, Y, Z
};

const SOMMET = `
attribute vec3 aPos; attribute vec3 aNor; attribute float aFace; attribute vec4 aCoul;
uniform mat4 uMVP;
varying vec3 vNor; varying vec3 vCoul; varying float vFace;
void main() {
  gl_Position = uMVP * vec4(aPos, 1.0);
  vNor = aNor; vCoul = aCoul.rgb; vFace = aFace;
}`;
// Three view-bound lights, as in SolidWorks, plus a specular highlight.
const COULEUR = `
precision highp float;
uniform vec3 uDroite; uniform vec3 uHaut; uniform vec3 uArriere;
uniform float uSel; uniform vec3 uChoisie; uniform float uTeinte;
varying vec3 vNor; varying vec3 vCoul; varying float vFace;
void main() {
  vec3 n = normalize(vNor);
  if (!gl_FrontFacing) n = -n;
  vec3 l1 = normalize(-0.45 * uDroite + 0.6 * uHaut + uArriere);
  vec3 l2 = normalize(0.8 * uDroite - 0.2 * uHaut + 0.5 * uArriere);
  vec3 l3 = normalize(0.1 * uDroite - 0.9 * uHaut + 0.3 * uArriere);
  float d = 0.55 * max(dot(n, l1), 0.0) + 0.25 * max(dot(n, l2), 0.0)
          + 0.10 * max(dot(n, l3), 0.0);
  float s = 0.28 * pow(max(dot(n, normalize(l1 + uArriere)), 0.0), 40.0);
  vec3 c = abs(vFace - uSel) < 0.5 ? uChoisie : mix(vec3(0.565, 0.792, 0.977), vCoul, uTeinte);
  gl_FragColor = vec4(c * (0.32 + d) + s, 1.0);
}`;
const RANG = `
precision highp float;
varying vec3 vNor; varying vec3 vCoul; varying float vFace;
void main() {
  float i = floor(vFace + 0.5) + 1.0;
  gl_FragColor = vec4(mod(i, 256.0), mod(floor(i / 256.0), 256.0),
                      floor(i / 65536.0), 255.0) / 255.0;
}`;
const TRAIT = `
attribute vec3 aPos; attribute float aFace;
uniform mat4 uMVP;
varying float vFace;
void main() { gl_Position = uMVP * vec4(aPos, 1.0); vFace = aFace; }`;
const ENCRE = `
precision highp float;
uniform float uSeul; uniform vec3 uEncre;
varying float vFace;
void main() {
  if (uSeul >= 0.0 && abs(vFace - uSeul) > 0.5) discard;
  gl_FragColor = vec4(uEncre, 1.0);
}`;

const ajout = (u, v, k = 1) => u.map((x, i) => x + k * v[i]);
const scal = (u, v) => u[0] * v[0] + u[1] * v[1] + u[2] * v[2];
const vect = (u, v) => [
  u[1] * v[2] - u[2] * v[1],
  u[2] * v[0] - u[0] * v[2],
  u[0] * v[1] - u[1] * v[0],
];
const unite = (u) => {
  const l = Math.hypot(...u) || 1;
  return u.map((x) => x / l);
};

// Rodrigues: v rotated by `angle` about the unit axis `axe`.
function rodrigues(v, axe, angle) {
  const c = Math.cos(angle),
    s = Math.sin(angle),
    k = scal(axe, v) * (1 - c);
  const x = vect(axe, v);
  return v.map((vi, i) => vi * c + x[i] * s + axe[i] * k);
}

// SolidWorks standard views, Y up (keys match ViewCube semantics).
const VUES = {
  front: [[1, 0, 0], [0, 1, 0]],
  back: [[-1, 0, 0], [0, 1, 0]],
  left: [[0, 0, 1], [0, 1, 0]],
  right: [[0, 0, -1], [0, 1, 0]],
  top: [[1, 0, 0], [0, 0, -1]],
  bottom: [[1, 0, 0], [0, 0, 1]],
  iso: [unite([1, 0, -1]), unite([-1, 2, -1])],
};

// Normals are smoothed within each face (a B-rep face is smooth) and broken
// between faces; drawn edges are face boundaries, i.e. edges belonging to a
// single facet. Shared vertices are matched by their bits: a mesh point is
// always written with the same three floats.
function preparer(faces, pos) {
  const n = pos.length / 3;
  const bits = new Uint32Array(pos.buffer, pos.byteOffset, pos.length);
  const nor = new Float32Array(pos.length);
  const face = new Float32Array(n);
  const coul = new Uint8Array(4 * n);
  const traits = [];
  const traitsFace = [];
  const boites = [];
  let k = 0;
  faces.forEach((f, rang) => {
    const c = (f.couleur || SW.defaut).map((x) => Math.round(255 * x));
    const ids = new Map(),
      somme = [],
      local = new Int32Array(3 * f.triangles);
    const plat = new Float32Array(3 * f.triangles);
    const bords = new Map();
    const b = [Infinity, Infinity, Infinity, -Infinity, -Infinity, -Infinity];
    for (let t = 0; t < f.triangles; t++) {
      const v0 = k + 3 * t,
        a = 3 * v0;
      const ux = pos[a + 3] - pos[a],
        uy = pos[a + 4] - pos[a + 1],
        uz = pos[a + 5] - pos[a + 2];
      const vx = pos[a + 6] - pos[a],
        vy = pos[a + 7] - pos[a + 1],
        vz = pos[a + 8] - pos[a + 2];
      const nx = uy * vz - uz * vy,
        ny = uz * vx - ux * vz,
        nz = ux * vy - uy * vx;
      const l = Math.hypot(nx, ny, nz) || 1;
      plat[3 * t] = nx / l;
      plat[3 * t + 1] = ny / l;
      plat[3 * t + 2] = nz / l;
      for (let j = 0; j < 3; j++) {
        const v = v0 + j,
          s = 3 * v;
        const cle = `${bits[s]},${bits[s + 1]},${bits[s + 2]}`;
        let id = ids.get(cle);
        if (id === undefined) {
          id = ids.size;
          ids.set(cle, id);
          somme.push(0, 0, 0);
        }
        local[3 * t + j] = id;
        somme[3 * id] += nx;
        somme[3 * id + 1] += ny;
        somme[3 * id + 2] += nz;
        face[v] = rang;
        coul.set(c, 4 * v);
        coul[4 * v + 3] = 255;
        for (let q = 0; q < 3; q++) {
          b[q] = Math.min(b[q], pos[s + q]);
          b[q + 3] = Math.max(b[q + 3], pos[s + q]);
        }
      }
      for (let j = 0; j < 3; j++) {
        const p = local[3 * t + j],
          q = local[3 * t + ((j + 1) % 3)];
        if (p === q) continue;
        const cle = p < q ? p * 4194304 + q : q * 4194304 + p;
        const e = bords.get(cle);
        if (e) e[0]++;
        else bords.set(cle, [1, v0 + j, v0 + ((j + 1) % 3)]);
      }
    }
    for (let t = 0; t < f.triangles; t++) {
      for (let j = 0; j < 3; j++) {
        const id = local[3 * t + j],
          s = 3 * (k + 3 * t + j);
        let x = somme[3 * id],
          y = somme[3 * id + 1],
          z = somme[3 * id + 2];
        const l = Math.hypot(x, y, z);
        const px = plat[3 * t],
          py = plat[3 * t + 1],
          pz = plat[3 * t + 2];
        // An average far from the facet normal means a crease: use flat.
        if (l < 1e-30 || (x * px + y * py + z * pz) / l < 0.5) {
          x = px;
          y = py;
          z = pz;
        } else {
          x /= l;
          y /= l;
          z /= l;
        }
        nor[s] = x;
        nor[s + 1] = y;
        nor[s + 2] = z;
      }
    }
    for (const [compte, p, q] of bords.values()) {
      if (compte !== 1) continue;
      traits.push(pos[3 * p], pos[3 * p + 1], pos[3 * p + 2], pos[3 * q], pos[3 * q + 1], pos[3 * q + 2]);
      traitsFace.push(rang, rang);
    }
    boites.push(b);
    k += 3 * f.triangles;
  });
  return {
    nor,
    face,
    coul,
    traits: new Float32Array(traits),
    traitsFace: new Float32Array(traitsFace),
    boites,
  };
}

export class CadWebGLRenderer {
  /**
   * @param canvas     main WebGL canvas (transparent; the background comes from CSS)
   * @param triedre    small 2D canvas for the reference triad
   * @param opts       { onPick(faceIndex, event), onReady(), onViewChanged() }
   */
  constructor(canvas, triedre, opts = {}) {
    this.canvas = canvas;
    this.triedre = triedre;
    this.opts = opts;
    this.gl = canvas.getContext("webgl", { antialias: true, alpha: true });
    this.b = null;
    this.n = 0;
    this.nTraits = 0;
    this.faces = [];
    this.boites = [];
    this.rayon = 1;
    this.sel = -1;
    this.survol = -1;
    this.attente = 0;
    this.minuteur = 0;
    this.fbo = null;
    this.fboL = 0;
    this.fboH = 0;
    // Style: face colours + black edges (occt) or plain shaded (standard).
    this.teinte = 1; // 1 = file colours, 0 = uniform CAD blue
    this.aretesOn = true;
    // An orthonormal frame (right, up, back) in part coordinates, a target
    // at the screen centre and a visible half-height, in mm.
    this.droite = [1, 0, 0];
    this.haut = [0, 1, 0];
    this.arriere = [0, 0, 1];
    this.cible = [0, 0, 0];
    this.demi = 1;
    if (!this.gl) return;
    const gl = this.gl;
    this.dessin = this.programme(SOMMET, COULEUR);
    this.rang = this.programme(SOMMET, RANG);
    this.trait = this.programme(TRAIT, ENCRE);
    gl.enable(gl.DEPTH_TEST);
    gl.depthFunc(gl.LEQUAL);
    this.brancher();
  }

  get ok() {
    return !!this.gl;
  }

  programme(sommet, fragment) {
    const gl = this.gl;
    const p = gl.createProgram();
    for (const [genre, source] of [
      [gl.VERTEX_SHADER, sommet],
      [gl.FRAGMENT_SHADER, fragment],
    ]) {
      const sh = gl.createShader(genre);
      gl.shaderSource(sh, source);
      gl.compileShader(sh);
      if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) throw gl.getShaderInfoLog(sh);
      gl.attachShader(p, sh);
    }
    ["aPos", "aNor", "aFace", "aCoul"].forEach((nom, i) => { gl.bindAttribLocation(p, i, nom); });
    gl.linkProgram(p);
    if (!gl.getProgramParameter(p, gl.LINK_STATUS)) throw gl.getProgramInfoLog(p);
    return p;
  }

  tampon(donnees) {
    const gl = this.gl;
    const b = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, b);
    gl.bufferData(gl.ARRAY_BUFFER, donnees, gl.STATIC_DRAW);
    return b;
  }

  lier(i, b, taille, genre, norme) {
    const gl = this.gl;
    gl.bindBuffer(gl.ARRAY_BUFFER, b);
    gl.enableVertexAttribArray(i);
    gl.vertexAttribPointer(i, taille, genre || gl.FLOAT, !!norme, 0, 0);
  }

  /** positions: Float32Array of centred triangle vertices; faces: the mesh
   * contract's faces ({triangles, couleur?, nom, corps}); rayon: mm. */
  setMesh(positions, faces, rayon) {
    const gl = this.gl;
    if (!gl) return;
    if (this.b) {
      for (const k of ["pos", "nor", "face", "coul", "traits", "traitsFace"]) {
        if (this.b[k]) gl.deleteBuffer(this.b[k]);
      }
    }
    this.positions = positions;
    this.faces = faces;
    const m = preparer(faces, positions);
    this.b = {
      pos: this.tampon(positions),
      nor: this.tampon(m.nor),
      face: this.tampon(m.face),
      coul: this.tampon(m.coul),
      traits: this.tampon(m.traits),
      traitsFace: this.tampon(m.traitsFace),
    };
    this.n = positions.length / 3;
    this.nTraits = m.traits.length / 3;
    this.boites = m.boites;
    this.rayon = rayon || 1;
    this.sel = -1;
    this.survol = -1;
    this.orienter("iso");
  }

  setStyle({ teinte, aretes } = {}) {
    if (teinte !== undefined) this.teinte = teinte;
    if (aretes !== undefined) this.aretesOn = aretes;
    this.redessiner();
  }

  // --- camera --------------------------------------------------------------

  tourner(axe, angle) {
    const r = (v) => rodrigues(v, axe, angle);
    this.droite = unite(r(this.droite));
    this.haut = unite(r(this.haut));
    this.arriere = vect(this.droite, this.haut);
    this.cible = r(this.cible); // the part centre is the origin (centred view)
    this.redessiner();
    this.opts.onViewChanged?.();
  }

  orienter(nom) {
    const vue = VUES[nom];
    if (!vue) return;
    const [d, h] = vue;
    this.droite = d.slice();
    this.haut = h.slice();
    this.arriere = vect(d, h);
    this.ajuster();
  }

  // Zoom to fit: the part's bounding boxes projected on the screen plane.
  ajuster() {
    const c = this.canvas,
      aspect = (c.clientWidth || 1) / (c.clientHeight || 1);
    let x0 = Infinity,
      x1 = -Infinity,
      y0 = Infinity,
      y1 = -Infinity;
    for (const b of this.boites) {
      if (!(b[0] <= b[3])) continue;
      for (let i = 0; i < 8; i++) {
        const p = [b[i & 1 ? 3 : 0], b[i & 2 ? 4 : 1], b[i & 4 ? 5 : 2]];
        const x = scal(p, this.droite),
          y = scal(p, this.haut);
        x0 = Math.min(x0, x);
        x1 = Math.max(x1, x);
        y0 = Math.min(y0, y);
        y1 = Math.max(y1, y);
      }
    }
    if (!(x0 <= x1)) {
      x0 = y0 = -this.rayon;
      x1 = y1 = this.rayon;
    }
    this.cible = ajout(ajout([0, 0, 0], this.droite, (x0 + x1) / 2), this.haut, (y0 + y1) / 2);
    this.demi = 1.08 * Math.max((y1 - y0) / 2, (x1 - x0) / 2 / aspect, this.rayon * 1e-3);
    this.redessiner();
    this.opts.onViewChanged?.();
  }

  // Column-major matrices, as WebGL reads them.
  matrice() {
    const c = this.canvas,
      aspect = c.width / c.height;
    const { droite: d, haut: h, arriere: a } = this;
    const recul = 4 * this.rayon + Math.hypot(...this.cible);
    const oeil = ajout(this.cible, a, recul);
    const vueM = [
      d[0], h[0], a[0], 0,
      d[1], h[1], a[1], 0,
      d[2], h[2], a[2], 0,
      -scal(d, oeil), -scal(h, oeil), -scal(a, oeil), 1,
    ];
    const sy = 1 / this.demi,
      sx = sy / aspect,
      loin = 2 * recul;
    const ortho = [sx, 0, 0, 0, 0, sy, 0, 0, 0, 0, -2 / loin, 0, 0, 0, -1, 1];
    const m = new Array(16).fill(0);
    for (let col = 0; col < 4; col++)
      for (let r = 0; r < 4; r++)
        for (let k = 0; k < 4; k++) m[4 * col + r] += ortho[4 * k + r] * vueM[4 * col + k];
    return m;
  }

  uniformes(prog, valeurs) {
    const gl = this.gl;
    for (const [nom, v] of Object.entries(valeurs)) {
      const l = gl.getUniformLocation(prog, nom);
      if (l === null) continue;
      if (typeof v === "number") gl.uniform1f(l, v);
      else if (v.length === 3) gl.uniform3fv(l, v);
      else gl.uniformMatrix4fv(l, false, new Float32Array(v));
    }
  }

  passeFaces(prog, mvp) {
    const gl = this.gl;
    gl.useProgram(prog);
    this.uniformes(prog, {
      uMVP: mvp,
      uSel: this.sel,
      uChoisie: SW.choisie,
      uDroite: this.droite,
      uHaut: this.haut,
      uArriere: this.arriere,
      uTeinte: this.teinte,
    });
    this.lier(0, this.b.pos, 3);
    this.lier(1, this.b.nor, 3);
    this.lier(2, this.b.face, 1);
    this.lier(3, this.b.coul, 4, gl.UNSIGNED_BYTE, true);
    gl.drawArrays(gl.TRIANGLES, 0, this.n);
  }

  aretes(mvp, seule, encre) {
    const gl = this.gl;
    if (!this.nTraits || !this.aretesOn) return;
    gl.useProgram(this.trait);
    this.uniformes(this.trait, { uMVP: mvp, uSeul: seule, uEncre: encre });
    gl.disableVertexAttribArray(1);
    gl.disableVertexAttribArray(3);
    this.lier(0, this.b.traits, 3);
    this.lier(2, this.b.traitsFace, 1);
    gl.drawArrays(gl.LINES, 0, this.nTraits);
  }

  redessiner() {
    if (this.attente) return;
    this.attente = requestAnimationFrame(() => {
      this.attente = 0;
      const c = this.canvas,
        r = window.devicePixelRatio || 1;
      const l = Math.round(c.clientWidth * r),
        h = Math.round(c.clientHeight * r);
      if (c.width !== l || c.height !== h) {
        c.width = l;
        c.height = h;
      }
      const gl = this.gl,
        mvp = this.matrice();
      gl.bindFramebuffer(gl.FRAMEBUFFER, null);
      gl.viewport(0, 0, c.width, c.height);
      gl.clearColor(0, 0, 0, 0); // the background gradient comes from CSS
      gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
      // Push faces back slightly so edges draw on top.
      gl.enable(gl.POLYGON_OFFSET_FILL);
      gl.polygonOffset(1, 1);
      this.passeFaces(this.dessin, mvp);
      gl.disable(gl.POLYGON_OFFSET_FILL);
      this.aretes(mvp, -1, SW.arete);
      if (this.survol >= 0 && this.survol !== this.sel) this.aretes(mvp, this.survol, SW.survol);
      this.triedreDessiner();
      this.opts.onDraw?.();
    });
  }

  // Reference triad, bottom left, as in SolidWorks.
  triedreDessiner() {
    const t = this.triedre,
      r = window.devicePixelRatio || 1;
    if (!t) return;
    const taille = 76;
    if (t.width !== taille * r) {
      t.width = t.height = taille * r;
    }
    const g = t.getContext("2d");
    g.setTransform(r, 0, 0, r, 0, 0);
    g.clearRect(0, 0, taille, taille);
    const o = taille / 2,
      long = 26;
    const axes = [
      [1, 0, 0],
      [0, 1, 0],
      [0, 0, 1],
    ].map((u, i) => ({
      x: scal(u, this.droite),
      y: scal(u, this.haut),
      z: scal(u, this.arriere),
      i,
    }));
    axes.sort((p, q) => p.z - q.z);
    g.lineWidth = 2;
    g.font = "bold 11px system-ui, sans-serif";
    for (const a of axes) {
      g.strokeStyle = g.fillStyle = SW.axes[a.i];
      g.beginPath();
      g.moveTo(o, o);
      g.lineTo(o + long * a.x, o - long * a.y);
      g.stroke();
      g.fillText("XYZ"[a.i], o + (long + 8) * a.x - 4, o - (long + 8) * a.y + 4);
    }
  }

  // Index of the face under canvas pixel (x, y), or -1.
  viser(x, y) {
    const gl = this.gl,
      c = this.canvas;
    if (!this.b) return -1;
    if (!this.fbo || this.fboL !== c.width || this.fboH !== c.height) {
      this.fbo = gl.createFramebuffer();
      const tex = gl.createTexture();
      gl.bindTexture(gl.TEXTURE_2D, tex);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, c.width, c.height, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
      const prof = gl.createRenderbuffer();
      gl.bindRenderbuffer(gl.RENDERBUFFER, prof);
      gl.renderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT16, c.width, c.height);
      gl.bindFramebuffer(gl.FRAMEBUFFER, this.fbo);
      gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0);
      gl.framebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, prof);
      this.fboL = c.width;
      this.fboH = c.height;
    }
    gl.bindFramebuffer(gl.FRAMEBUFFER, this.fbo);
    gl.viewport(0, 0, c.width, c.height);
    gl.clearColor(0, 0, 0, 0);
    gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    this.passeFaces(this.rang, this.matrice());
    const px = new Uint8Array(4);
    gl.readPixels(x, c.height - 1 - y, 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, px);
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    return px[0] + 256 * px[1] + 65536 * px[2] - 1;
  }

  /** Face index under a client-space event, or -1. */
  sous(e) {
    const c = this.canvas,
      r = c.getBoundingClientRect(),
      d = c.width / r.width;
    return this.viser(
      Math.floor((e.clientX - r.left) * d),
      Math.floor((e.clientY - r.top) * d)
    );
  }

  /** The world point (mm) under a client-space event, or null: an ortho ray
   * cast from in front of the model (beyond its bounds, toward the camera
   * side) into the screen, against every triangle, Möller–Trumbore. Starting
   * on the target plane would begin inside the model and hit back faces. */
  pointSous(e) {
    if (!this.positions) return null;
    const c = this.canvas,
      r = c.getBoundingClientRect();
    const aspect = c.clientWidth / c.clientHeight;
    const nx = (((e.clientX - r.left) / r.width) * 2 - 1) * this.demi * aspect;
    const ny = (1 - ((e.clientY - r.top) / r.height) * 2) * this.demi;
    const origine = ajout(
      ajout(
        ajout(this.cible, this.droite, nx),
        this.haut,
        ny
      ),
      this.arriere,
      2 * (this.rayon || 1)
    );
    const dir = this.arriere.map((x) => -x);
    const p = this.positions;
    let best = null;
    let bestT = Infinity;
    for (let t = 0; t + 8 < p.length; t += 9) {
      const ax = p[t], ay = p[t + 1], az = p[t + 2];
      const e1x = p[t + 3] - ax, e1y = p[t + 4] - ay, e1z = p[t + 5] - az;
      const e2x = p[t + 6] - ax, e2y = p[t + 7] - ay, e2z = p[t + 8] - az;
      const px = dir[1] * e2z - dir[2] * e2y, py = dir[2] * e2x - dir[0] * e2z, pz = dir[0] * e2y - dir[1] * e2x;
      const det = e1x * px + e1y * py + e1z * pz;
      if (det > -1e-12 && det < 1e-12) continue;
      const inv = 1 / det;
      const tx = origine[0] - ax, ty = origine[1] - ay, tz = origine[2] - az;
      const u = (tx * px + ty * py + tz * pz) * inv;
      if (u < 0 || u > 1) continue;
      const qx = ty * e1z - tz * e1y, qy = tz * e1x - tx * e1z, qz = tx * e1y - ty * e1x;
      const v = (dir[0] * qx + dir[1] * qy + dir[2] * qz) * inv;
      if (v < 0 || u + v > 1) continue;
      const tt = (e2x * qx + e2y * qy + e2z * qz) * inv;
      if (tt > 0 && tt < bestT) {
        bestT = tt;
        best = [origine[0] + tt * dir[0], origine[1] + tt * dir[1], origine[2] + tt * dir[2]];
      }
    }
    return best;
  }

  /** Project a world point to container pixel coordinates (for labels). */
  ecran(p) {
    const m = this.matrice();
    const x = m[0] * p[0] + m[4] * p[1] + m[8] * p[2] + m[12];
    const y = m[1] * p[0] + m[5] * p[1] + m[9] * p[2] + m[13];
    const w = m[3] * p[0] + m[7] * p[1] + m[11] * p[2] + m[15];
    return {
      x: ((x / w) * 0.5 + 0.5) * this.canvas.clientWidth,
      y: (1 - (y / w) * 0.5 - 0.5) * this.canvas.clientHeight,
    };
  }

  /** The camera's view direction (for the view cube). */
  direction() {
    return { x: -this.arriere[0], y: -this.arriere[1], z: -this.arriere[2] };
  }

  select(rang) {
    this.sel = rang;
    this.redessiner();
  }

  // Zoom keeps the point under the cursor fixed (nx, ny in [−1, 1]).
  zoomer(f, nx = 0, ny = 0) {
    const c = this.canvas,
      aspect = c.clientWidth / c.clientHeight;
    this.cible = ajout(
      ajout(this.cible, this.droite, (1 - f) * this.demi * nx * aspect),
      this.haut,
      (1 - f) * this.demi * ny
    );
    this.demi *= f;
    this.redessiner();
  }

  deplacer(dx, dy) {
    const k = (2 * this.demi) / this.canvas.clientHeight;
    this.cible = ajout(ajout(this.cible, this.droite, -k * dx), this.haut, k * dy);
    this.redessiner();
  }

  // SolidWorks mouse: middle button rotates, with Ctrl pans, with Shift
  // zooms, with Alt rolls in the screen plane; middle double-click fits.
  // The wheel zooms towards the cursor. Left click selects a face (routed to
  // opts.onPick so the host can intercept it while measuring). Without a
  // middle button, left drag rotates and right drag pans.
  brancher() {
    const c = this.canvas;
    let glisse = null,
      milieu = 0;
    c.tabIndex = 0;
    c.style.outline = "none";
    c.addEventListener("contextmenu", (e) => e.preventDefault());
    c.addEventListener("mousedown", (e) => {
      if (e.button === 1) e.preventDefault();
    });
    c.addEventListener("pointerdown", (e) => {
      c.focus();
      try {
        c.setPointerCapture(e.pointerId);
      } catch {
        /* synthetic events have no active pointer */
      }
      if (e.button === 1) {
        e.preventDefault(); // no browser autoscroll
        const t = performance.now();
        if (t - milieu < 350) {
          this.ajuster();
          milieu = 0;
          return;
        }
        milieu = t;
      }
      const mode =
        e.button === 1
          ? e.ctrlKey
            ? "deplacer"
            : e.shiftKey
              ? "zoomer"
              : e.altKey
                ? "pivoter"
                : "tourner"
          : e.button === 2 || e.shiftKey
            ? "deplacer"
            : "tourner";
      glisse = { x: e.clientX, y: e.clientY, x0: e.clientX, y0: e.clientY, mode, bouton: e.button };
      this.survol = -1;
    });
    c.addEventListener("pointermove", (e) => {
      if (!glisse) {
        this.survoler(c, e);
        return;
      }
      const dx = e.clientX - glisse.x,
        dy = e.clientY - glisse.y;
      glisse.x = e.clientX;
      glisse.y = e.clientY;
      if (glisse.mode === "deplacer") this.deplacer(dx, dy);
      else if (glisse.mode === "zoomer") this.zoomer(Math.exp(dy * 0.005));
      else if (glisse.mode === "pivoter") this.tourner(this.arriere, dx * 0.008);
      else {
        const haut = this.haut.slice(),
          droite = this.droite.slice();
        if (dx) this.tourner(haut, -dx * 0.008);
        if (dy) this.tourner(droite, -dy * 0.008);
      }
    });
    c.addEventListener("pointerup", (e) => {
      if (
        glisse &&
        glisse.bouton === 0 &&
        Math.hypot(e.clientX - glisse.x0, e.clientY - glisse.y0) < 4
      ) {
        this.opts.onPick?.(this.sous(e), e);
      }
      glisse = null;
    });
    c.addEventListener("pointerleave", () => {
      if (this.survol >= 0 && !glisse) {
        this.survol = -1;
        this.redessiner();
      }
    });
    c.addEventListener(
      "wheel",
      (e) => {
        e.preventDefault();
        const r = c.getBoundingClientRect();
        this.zoomer(
          Math.exp(-e.deltaY * 0.0015),
          ((e.clientX - r.left) / r.width) * 2 - 1,
          1 - ((e.clientY - r.top) / r.height) * 2
        );
      },
      { passive: false }
    );
    // SolidWorks keyboard shortcuts, while the canvas has focus.
    c.addEventListener("keydown", (e) => {
      const fleches = {
        ArrowLeft: [-1, 0],
        ArrowRight: [1, 0],
        ArrowUp: [0, -1],
        ArrowDown: [0, 1],
      };
      const vues = { 1: "front", 2: "back", 3: "left", 4: "right", 5: "top", 6: "bottom", 7: "iso" };
      let fait = true;
      if (e.key in fleches) {
        const [x, y] = fleches[e.key];
        const pas = e.shiftKey ? Math.PI / 2 : SW.pas;
        if (e.ctrlKey) this.deplacer(-x * 20, -y * 20);
        else if (e.altKey && x) this.tourner(this.arriere, x * pas);
        else if (x) this.tourner(this.haut.slice(), -x * pas);
        else this.tourner(this.droite.slice(), -y * pas);
      } else if (e.key === "f" || e.key === "F") this.ajuster();
      else if (e.key === "z") this.zoomer(1.25);
      else if (e.key === "Z") this.zoomer(0.8);
      else if (e.ctrlKey && vues[e.key]) this.orienter(vues[e.key]);
      else if (e.key === "Escape") this.opts.onPick?.(-1, e);
      else fait = false;
      if (fait) e.preventDefault();
    });
    this.resizeObserver = new ResizeObserver(() => this.redessiner());
    this.resizeObserver.observe(c);
  }

  // Dynamic highlight. An offscreen pass per mouse move is costly, so wait
  // for the mouse to settle.
  survoler(_c, e) {
    clearTimeout(this.minuteur);
    this.minuteur = setTimeout(() => {
      const rang = this.sous(e);
      if (rang !== this.survol) {
        this.survol = rang;
        this.redessiner();
      }
    }, 40);
  }

  dispose() {
    if (this.attente) cancelAnimationFrame(this.attente);
    clearTimeout(this.minuteur);
    this.resizeObserver?.disconnect();
    if (this.gl && this.b) {
      for (const k of ["pos", "nor", "face", "coul", "traits", "traitsFace"]) {
        if (this.b[k]) this.gl.deleteBuffer(this.b[k]);
      }
    }
    if (this.gl) {
      if (this.dessin) this.gl.deleteProgram(this.dessin);
      if (this.rang) this.gl.deleteProgram(this.rang);
      if (this.trait) this.gl.deleteProgram(this.trait);
    }
    if (this.fbo && this.gl) this.gl.deleteFramebuffer(this.fbo);
    this.b = null;
    this.gl = null;
  }
}
