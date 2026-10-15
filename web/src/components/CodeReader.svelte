<script lang="ts">
  import { mount, onMount, unmount, type Snippet } from "svelte";
  import {
    EditorState,
    StateEffect,
    StateField,
    Text,
  } from "@codemirror/state";
  import {
    EditorView,
    lineNumbers,
    highlightActiveLine,
    highlightSpecialChars,
    keymap,
    GutterMarker,
    gutter,
    Decoration,
    WidgetType,
  } from "@codemirror/view";
  import {
    search,
    searchKeymap,
    openSearchPanel,
    gotoLine,
  } from "@codemirror/search";
  import {
    syntaxHighlighting,
    defaultHighlightStyle,
  } from "@codemirror/language";
  import { Button } from "$lib/components/ui/button";
  import type { CodeBlock, CodeLine } from "../code-diff-model";
  import CodeDiffGap from "./CodeDiffGap.svelte";
  let {
    text,
    path,
    mapping,
    gaps,
    onexpand,
    canExpand = () => true,
    onselect,
    lineAction,
    restoreLine = 1,
  }: {
    text: string;
    path: string;
    mapping?: (CodeLine | null)[];
    gaps?: Map<number, CodeBlock>;
    onexpand?: (gapId: string, direction: "up" | "down" | "all") => void;
    canExpand?: () => boolean;
    onselect: (start: number, end: number) => void;
    lineAction?: Snippet<[row: number]>;
    restoreLine?: number;
  } = $props();
  let host: HTMLDivElement;
  let view: EditorView | undefined;
  let anchor: number | undefined;
  let coarse = $state(false);
  let actionLine = $state<number>();
  let actionTop = $state<number>();
  const pickLine = StateEffect.define<number | undefined>();
  export function clearLineAction() {
    actionLine = undefined;
    actionTop = undefined;
    view?.dispatch({ effects: pickLine.of(undefined) });
  }
  function positionLineAction() {
    if (!view || actionLine === undefined || !coarse) return;
    view.requestMeasure({
      key: positionLineAction,
      read(editor) {
        if (actionLine === undefined || !coarse) return;
        const block = editor.lineBlockAt(
          editor.state.doc.line(actionLine).from,
        );
        const height = host.clientHeight;
        const center =
          editor.documentTop +
          block.top +
          block.height / 2 -
          host.getBoundingClientRect().top;
        if (center < 0 || center > height) return;
        return Math.max(22, Math.min(height - 22, center));
      },
      write(top) {
        actionTop = top;
      },
    });
  }
  export function line() {
    if (!view) return 1;
    const top = view.scrollDOM.getBoundingClientRect().top - view.documentTop;
    return view.state.doc.lineAt(view.lineBlockAtHeight(top).from).number;
  }
  function select(number: number, shift: boolean) {
    if (coarse) {
      anchor = undefined;
      actionTop = undefined;
      actionLine = number;
      if (view) {
        view.dispatch({
          selection: { anchor: view.state.doc.line(number).from },
          effects: pickLine.of(number),
        });
      }
      onselect(number, number);
      positionLineAction();
      return;
    }
    if (shift && anchor !== undefined) {
      onselect(Math.min(anchor, number), Math.max(anchor, number));
      anchor = undefined;
    } else {
      anchor = number;
      onselect(number, number);
    }
  }
  class NumberMarker extends GutterMarker {
    oldLine: string;
    newLine: string;
    constructor(oldLine: string, newLine: string) {
      super();
      this.oldLine = oldLine;
      this.newLine = newLine;
    }
    eq(other: NumberMarker) {
      return other.oldLine === this.oldLine && other.newLine === this.newLine;
    }
    toDOM() {
      const element = document.createElement("span");
      const column = (text: string) => {
        const cell = document.createElement("span");
        cell.className = "diff-gutter-number";
        cell.textContent = text;
        return cell;
      };
      // The separator keeps textContent as "old new" for selection/copy.
      element.append(column(this.oldLine), " ", column(this.newLine));
      return element;
    }
  }
  class GapWidget extends WidgetType {
    component?: ReturnType<typeof mount>;
    gap: CodeBlock;
    up: boolean;
    down: boolean;
    constructor(gap: CodeBlock, up: boolean, down: boolean) {
      super();
      this.gap = gap;
      this.up = up;
      this.down = down;
    }
    toDOM() {
      const element = document.createElement("div");
      this.component = mount(CodeDiffGap, {
        target: element,
        props: {
          gap: this.gap,
          up: this.up,
          down: this.down,
          canExpand,
          onexpand: (direction) => onexpand?.(this.gap.gapId, direction),
        },
      });
      return element;
    }
    get estimatedHeight() {
      return 36;
    }
    coordsAt(dom: HTMLElement) {
      const rect = dom.getBoundingClientRect();
      return {
        left: rect.left,
        right: rect.left,
        top: rect.top,
        bottom: rect.bottom,
      };
    }
    destroy() {
      if (this.component) void unmount(this.component);
    }
  }
  onMount(() => {
    let cancelled = false;
    const pointer = matchMedia("(pointer: coarse)");
    const updatePointer = () => {
      coarse = pointer.matches;
      if (!coarse) clearLineAction();
    };
    updatePointer();
    pointer.addEventListener("change", updatePointer);
    const selectionChanged = () => {
      const selection = document.getSelection();
      if (
        selection &&
        !selection.isCollapsed &&
        host.contains(selection.anchorNode)
      ) {
        // A transaction here would reset the browser's native selection.
        actionLine = undefined;
        actionTop = undefined;
      }
    };
    document.addEventListener("selectionchange", selectionChanged);
    const extensions = [
      EditorState.readOnly.of(true),
      EditorView.editable.of(false),
      search(),
      keymap.of(searchKeymap),
      syntaxHighlighting(defaultHighlightStyle),
      highlightActiveLine(),
      highlightSpecialChars(),
      EditorView.theme({
        "&": { height: "100%", fontSize: "13px" },
        ".cm-scroller": {
          overflow: "auto",
          fontFamily: "var(--font-mono, monospace)",
        },
        ".cm-content": { minWidth: "100%" },
        ".cm-gutters": { cursor: "pointer" },
        ".code-line-picked": {
          boxShadow: "inset 3px 0 var(--primary)",
          outline: "1px solid var(--primary)",
          outlineOffset: "-1px",
        },
      }),
      EditorView.domEventHandlers({
        click(event, editor) {
          if (!coarse || !lineAction) return false;
          const target = event.target;
          if (!(target instanceof Element) || !target.closest(".cm-line"))
            return false;
          if (document.getSelection()?.isCollapsed === false) return false;
          const pos = editor.posAtCoords({
            x: event.clientX,
            y: event.clientY,
          });
          if (pos === null) return false;
          const row = editor.state.doc.lineAt(pos).number;
          if (
            mapping &&
            (!mapping[row - 1] || mapping[row - 1]?.kind === "marker")
          ) {
            clearLineAction();
            return false;
          }
          select(row, false);
          return false;
        },
        scroll() {
          positionLineAction();
          return false;
        },
      }),
      mapping
        ? gutter({
            class: "diff-gutter",
            lineMarker: (v, l) => {
              const m = mapping?.[v.state.doc.lineAt(l.from).number - 1];
              return new NumberMarker(
                m?.oldLine ? String(m.oldLine) : "",
                m?.newLine ? String(m.newLine) : "",
              );
            },
            domEventHandlers: {
              click: (v, l, e) => {
                if (!mapping?.[v.state.doc.lineAt(l.from).number - 1])
                  return false;
                select(
                  v.state.doc.lineAt(l.from).number,
                  (e as MouseEvent).shiftKey,
                );
                return true;
              },
            },
          })
        : lineNumbers({
            domEventHandlers: {
              click: (v, l, e) => {
                select(
                  v.state.doc.lineAt(l.from).number,
                  (e as MouseEvent).shiftKey,
                );
                return true;
              },
            },
          }),
      EditorView.editorAttributes.of({
        style: `--diff-gutter-digits: ${
          String(
            (mapping ?? []).reduce(
              (max, line) =>
                line ? Math.max(max, line.oldLine, line.newLine) : max,
              0,
            ),
          ).length
        }ch`,
      }),
      StateField.define({
        create: () => Decoration.none,
        update(value, transaction) {
          for (const effect of transaction.effects) {
            if (effect.is(pickLine)) {
              return effect.value === undefined
                ? Decoration.none
                : Decoration.set([
                    Decoration.line({ class: "code-line-picked" }).range(
                      transaction.state.doc.line(effect.value).from,
                    ),
                  ]);
            }
          }
          return value;
        },
        provide: (field) => EditorView.decorations.from(field),
      }),
      StateField.define({
        create(state) {
          return Decoration.set(
            mapping
              ? mapping.flatMap((line, i) => {
                  const gap = gaps?.get(i + 1);
                  if (gap && onexpand) {
                    const row = state.doc.line(i + 1);
                    return [
                      Decoration.replace({
                        block: true,
                        widget: new GapWidget(
                          gap,
                          i + 1 < state.doc.lines || i === 0,
                          i > 0,
                        ),
                      }).range(row.from, row.to),
                    ];
                  }
                  return line && (line.kind === "add" || line.kind === "delete")
                    ? [
                        Decoration.line({
                          class: `code-diff-${line.kind}`,
                        }).range(state.doc.line(i + 1).from),
                      ]
                    : [];
                })
              : [],
          );
        },
        update(value) {
          return value;
        },
        provide: (field) => EditorView.decorations.from(field),
      }),
      EditorView.updateListener.of((u) => {
        if (u.geometryChanged || u.viewportChanged) positionLineAction();
        if (u.selectionSet) {
          const range = u.state.selection.main;
          if (!range.empty)
            onselect(
              u.state.doc.lineAt(range.from).number,
              u.state.doc.lineAt(Math.max(range.from, range.to - 1)).number,
            );
        }
      }),
    ];
    view = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: Text.of(text.split(/\r?\n/)),
        extensions,
      }),
    });
    const target = Math.min(Math.max(restoreLine, 1), view.state.doc.lines);
    view.dispatch({
      effects: EditorView.scrollIntoView(view.state.doc.line(target).from, {
        y: "start",
        yMargin: 0,
      }),
    });
    void (async () => {
      const extension = path.split(".").at(-1)?.toLowerCase();
      let language;
      if (["js", "jsx", "ts", "tsx", "mjs", "svelte"].includes(extension ?? ""))
        language = (await import("@codemirror/lang-javascript")).javascript({
          typescript: extension?.startsWith("ts"),
          jsx: extension?.endsWith("x"),
        });
      else if (extension === "py")
        language = (await import("@codemirror/lang-python")).python();
      else if (extension === "json")
        language = (await import("@codemirror/lang-json")).json();
      else if (extension === "go")
        language = (await import("@codemirror/lang-go")).go();
      else if (extension === "rs")
        language = (await import("@codemirror/lang-rust")).rust();
      else if (extension === "html")
        language = (await import("@codemirror/lang-html")).html();
      else if (extension === "css")
        language = (await import("@codemirror/lang-css")).css();
      if (!cancelled && view && language) {
        if (!cancelled)
          view.dispatch({ effects: StateEffect.appendConfig.of(language) });
      }
    })().catch(() => {
      /* Plain text remains readable if language loading fails. */
    });
    return () => {
      cancelled = true;
      pointer.removeEventListener("change", updatePointer);
      document.removeEventListener("selectionchange", selectionChanged);
      view?.destroy();
      view = undefined;
    };
  });
</script>

<div class="flex min-h-0 flex-1 flex-col overflow-hidden">
  <div class="flex shrink-0 gap-2 border-b px-2 py-1">
    <Button
      variant="ghost"
      size="sm"
      onclick={() => view && openSearchPanel(view)}>文件内查找</Button
    >
    <Button variant="ghost" size="sm" onclick={() => view && gotoLine(view)}
      >跳转行</Button
    >
    <span class="self-center text-xs text-muted-foreground">
      {coarse
        ? "点击代码行，再点右侧 ＋ 发送位置"
        : "点击行号；Shift 点击选择终点"}
    </span>
  </div>
  <div class="relative min-h-0 flex-1 overflow-hidden">
    <div
      bind:this={host}
      class="code-reader h-full overflow-hidden"
      aria-label="只读代码"
    ></div>
    {#if coarse && actionLine !== undefined && actionTop !== undefined && lineAction}
      <div
        class="absolute right-1 z-10 -translate-y-1/2"
        style:top={`${actionTop}px`}
      >
        {@render lineAction(actionLine)}
      </div>
    {/if}
  </div>
</div>
