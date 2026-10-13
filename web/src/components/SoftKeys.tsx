import { useState } from "preact/hooks";
import type { Modifiers } from "../input";
const keys: [string, string][] = [
  ["Esc", "\x1b"],
  ["Tab", "\t"],
  ["↑", "\x1b[A"],
  ["↓", "\x1b[B"],
  ["←", "\x1b[D"],
  ["→", "\x1b[C"],
  ["Home", "\x1b[H"],
  ["End", "\x1b[F"],
  ["PgUp", "\x1b[5~"],
  ["PgDn", "\x1b[6~"],
];
const combinations: [string, string][] = [
  ["Ctrl+C", "\x03"],
  ["Ctrl+D", "\x04"],
  ["Ctrl+Z", "\x1a"],
  ["Ctrl+L", "\x0c"],
  ["Ctrl+R", "\x12"],
];
export function SoftKeys({
  send,
  modifiers,
  toggle,
  paste,
  keyboard,
}: {
  send: (input: string) => void;
  modifiers: Modifiers;
  toggle: (key: keyof Modifiers) => void;
  paste: () => void;
  keyboard: () => void;
}) {
  const [collapsed, setCollapsed] = useState(false);
  return (
    <aside
      class={"soft-keys" + (collapsed ? " collapsed" : "")}
      aria-label="终端软键"
    >
      <button
        class="soft-toggle"
        aria-label={collapsed ? "展开软键栏" : "折叠软键栏"}
        onPointerDown={(e) => e.preventDefault()}
        onClick={() => setCollapsed(!collapsed)}
      >
        {collapsed ? "⌃" : "⌄"}
      </button>
      {!collapsed && (
        <>
          <div class="soft-row">
            {keys.slice(0, 2).map(([label, input]) => (
              <button
                onPointerDown={(e) => e.preventDefault()}
                onClick={() => send(input)}
              >
                {label}
              </button>
            ))}
            {(["ctrl", "alt"] as const).map((key) => (
              <button
                class={modifiers[key] ? "selected" : ""}
                aria-pressed={modifiers[key]}
                onPointerDown={(e) => e.preventDefault()}
                onClick={() => toggle(key)}
              >
                {key === "ctrl" ? "Ctrl" : "Alt"}
              </button>
            ))}
            {keys.slice(2).map(([label, input]) => (
              <button
                onPointerDown={(e) => e.preventDefault()}
                onClick={() => send(input)}
              >
                {label}
              </button>
            ))}
            <button onPointerDown={(e) => e.preventDefault()} onClick={paste}>
              粘贴
            </button>
            <button
              aria-label="唤起或收起键盘"
              onPointerDown={(e) => e.preventDefault()}
              onClick={keyboard}
            >
              ⌨
            </button>
          </div>
          <div class="soft-row combinations">
            {combinations.map(([label, input]) => (
              <button
                onPointerDown={(e) => e.preventDefault()}
                onClick={() => send(input)}
              >
                {label}
              </button>
            ))}
          </div>
        </>
      )}
    </aside>
  );
}
