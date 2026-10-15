import { HighlightStyle } from "@codemirror/language";
import { tags } from "@lezer/highlight";

/**
 * Mirrors CodeMirror's default highlight tags, but reads colors from
 * `--code-*` variables so `.dark` swaps the palette without reconfiguring.
 */
export const codeHighlightStyle = HighlightStyle.define([
  { tag: tags.meta, color: "var(--code-meta)" },
  { tag: tags.link, textDecoration: "underline" },
  { tag: tags.heading, textDecoration: "underline", fontWeight: "bold" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.strong, fontWeight: "bold" },
  { tag: tags.strikethrough, textDecoration: "line-through" },
  { tag: tags.keyword, color: "var(--code-keyword)" },
  {
    tag: [
      tags.atom,
      tags.bool,
      tags.url,
      tags.contentSeparator,
      tags.labelName,
    ],
    color: "var(--code-atom)",
  },
  { tag: [tags.literal, tags.inserted], color: "var(--code-literal)" },
  { tag: [tags.string, tags.deleted], color: "var(--code-string)" },
  {
    tag: [tags.regexp, tags.escape, tags.special(tags.string)],
    color: "var(--code-regexp)",
  },
  {
    tag: tags.definition(tags.variableName),
    color: "var(--code-definition)",
  },
  { tag: tags.local(tags.variableName), color: "var(--code-local)" },
  { tag: [tags.typeName, tags.namespace], color: "var(--code-type)" },
  { tag: tags.className, color: "var(--code-class)" },
  {
    tag: [tags.special(tags.variableName), tags.macroName],
    color: "var(--code-special)",
  },
  {
    tag: tags.definition(tags.propertyName),
    color: "var(--code-property)",
  },
  { tag: tags.comment, color: "var(--code-comment)" },
  { tag: tags.invalid, color: "var(--code-invalid)" },
]);
