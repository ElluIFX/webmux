import { stored, store } from "./api";
import { ThemeSchema, type Theme } from "./schemas";

const system = matchMedia("(prefers-color-scheme: light)");
let choice: Theme | undefined;

function savedTheme(): Theme | undefined {
  const result = ThemeSchema.safeParse(stored("theme"));
  return result.success ? result.data : undefined;
}

function applyTheme() {
  const dark = choice ? choice === "dark" : !system.matches;
  const root = document.documentElement;
  root.classList.toggle("dark", dark);
  const meta = document.querySelector<HTMLMetaElement>(
    'meta[name="theme-color"]',
  );
  if (meta)
    meta.content = getComputedStyle(root)
      .getPropertyValue("--background")
      .trim();
}

export function initializeTheme() {
  choice = savedTheme();
  applyTheme();
  system.addEventListener("change", () => {
    if (!choice) applyTheme();
  });
  window.addEventListener("storage", (event) => {
    if (event.key === "webmux.theme" || event.key === null) {
      choice = savedTheme();
      applyTheme();
    }
  });
}

export function toggleTheme() {
  choice = document.documentElement.classList.contains("dark")
    ? "light"
    : "dark";
  store("theme", choice);
  applyTheme();
}
