(() => {
  const root = document.documentElement;
  const dark = matchMedia("(prefers-color-scheme: dark)");
  const current = () => root.dataset.theme || (dark.matches ? "dark" : "light");

  document.querySelector(".theme-toggle")?.addEventListener("click", () => {
    const next = current() === "dark" ? "light" : "dark";
    root.dataset.theme = next;
    try {
      localStorage.setItem("theme", next);
    } catch {}
  });

  document.querySelectorAll("[data-copy]").forEach((button) => {
    button.addEventListener("click", async () => {
      try {
        await navigator.clipboard.writeText(button.dataset.copy);
        button.textContent = "copied";
      } catch {
        button.textContent = "select and copy";
      }
      setTimeout(() => (button.textContent = "copy"), 1600);
    });
  });
})();

(() => {
  const title = document.querySelector("h1.scramble");
  if (!title || matchMedia("(prefers-reduced-motion: reduce)").matches) return;

  const text = title.dataset.text;
  const shown = title.querySelector(".q-text");
  const GLYPHS = "abcdefghijklmnopqrstuvwxyz0123456789";
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const glyph = () => GLYPHS[Math.floor(Math.random() * GLYPHS.length)];
  const scramble = (s) => s.replace(/[^\s,.]/g, glyph);
  const FRAME = 45;

  async function scrambleOut(ms) {
    const frames = Math.round(ms / FRAME);
    for (let f = 1; f <= frames; f++) {
      const gone = Math.floor((text.length * f) / frames);
      shown.textContent = text.slice(0, text.length - gone) + scramble(text.slice(text.length - gone));
      await sleep(FRAME);
    }
  }

  async function hold(ms) {
    for (let t = 0; t < ms; t += FRAME * 2) {
      shown.textContent = scramble(text);
      await sleep(FRAME * 2);
    }
  }

  async function decode(ms) {
    const frames = Math.round(ms / FRAME);
    for (let f = 1; f <= frames; f++) {
      const settled = Math.floor((text.length * f) / frames);
      shown.textContent = text.slice(0, settled) + scramble(text.slice(settled));
      await sleep(FRAME);
    }
    shown.textContent = text;
  }

  (async () => {
    await sleep(2200);
    for (;;) {
      title.classList.add("scrambling");
      await scrambleOut(650);
      await hold(1100);
      await decode(1300);
      title.classList.remove("scrambling");
      await sleep(6500);
    }
  })();
})();

(() => {
  const figure = document.querySelector(".chain");
  if (!figure || matchMedia("(prefers-reduced-motion: reduce)").matches) return;

  const cells = [...figure.querySelectorAll(".link")];
  const vault = figure.querySelector("[data-vault]");
  const GLYPHS = "abcdefghijklmnopqrstuvwxyz0123456789";
  const DOTS = [12, 8, 9, 11];
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const glyph = () => GLYPHS[Math.floor(Math.random() * GLYPHS.length)];
  const scramble = (text) => text.replace(/\S/g, glyph);

  const parts = (cell) => ({
    q: cell.querySelector(".q-text"),
    text: cell.querySelector(".q").dataset.text,
    typed: cell.querySelector(".typed"),
    state: cell.querySelector(".state"),
  });

  async function decode(cell, ms) {
    const { q, text } = parts(cell);
    const frames = Math.round(ms / 40);
    for (let f = 1; f <= frames; f++) {
      const settled = Math.floor((text.length * f) / frames);
      q.textContent = text.slice(0, settled) + scramble(text.slice(settled));
      if (f === Math.ceil(frames / 3)) cell.classList.remove("sealed");
      await sleep(40);
    }
    q.textContent = text;
  }

  async function encode(cell, ms) {
    const { q, text } = parts(cell);
    const frames = Math.round(ms / 40);
    for (let f = 1; f <= frames; f++) {
      const scrambled = Math.floor((text.length * f) / frames);
      q.textContent = text.slice(0, text.length - scrambled) + scramble(text.slice(text.length - scrambled));
      await sleep(40);
    }
    cell.classList.add("sealed");
  }

  async function type(cell, count) {
    const { typed } = parts(cell);
    for (let i = 1; i <= count; i++) {
      typed.textContent = "•".repeat(i);
      await sleep(70 + Math.random() * 90);
    }
  }

  function seal(cell) {
    const { q, text, typed, state } = parts(cell);
    cell.classList.remove("done", "now");
    typed.textContent = "";
    if (text) {
      cell.classList.add("sealed");
      q.textContent = scramble(text);
      state.textContent = "sealed";
    }
  }

  function open(cell) {
    const { state } = parts(cell);
    state.textContent = state.dataset.open;
  }

  function setVault(isOpen) {
    vault.textContent = isOpen ? "vault · open" : "vault · sealed";
    vault.classList.toggle("open", isOpen);
  }

  let running = false;

  async function loop() {
    if (running) return;
    running = true;
    for (;;) {
      await Promise.all(cells.slice(1).filter((c) => !c.classList.contains("sealed")).map((c) => encode(c, 500)));
      cells.forEach(seal);
      setVault(false);
      open(cells[0]);
      await sleep(900);

      for (let i = 0; i < cells.length; i++) {
        const cell = cells[i];
        cell.classList.add("now");
        await sleep(350);
        await type(cell, DOTS[i]);
        await sleep(250);
        cell.classList.remove("now");
        cell.classList.add("done");
        const next = cells[i + 1];
        if (next) {
          await sleep(200);
          await decode(next, 900);
          open(next);
          await sleep(500);
        }
      }
      setVault(true);
      await sleep(3200);
    }
  }

  new IntersectionObserver((entries) => {
    if (entries.some((e) => e.isIntersecting)) loop();
  }).observe(figure);
})();
