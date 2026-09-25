const KEY = "theme";

const chosen = (): string | null => {
  try {
    const value = localStorage.getItem(KEY);
    return value === "light" || value === "dark" ? value : null;
  } catch {
    return null;
  }
};

export function flipTheme(): "light" | "dark" {
  const root = document.documentElement;
  const now = root.dataset.theme || (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
  const next = now === "dark" ? "light" : "dark";
  root.dataset.theme = next;
  try {
    localStorage.setItem(KEY, next);
  } catch {
    // the choice lasts as long as the tab does
  }
  return next;
}

export function rememberTheme() {
  const picked = chosen();
  if (picked) document.documentElement.dataset.theme = picked;
}
