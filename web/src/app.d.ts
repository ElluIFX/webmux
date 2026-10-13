import "svelte/elements";

declare module "svelte/elements" {
  interface HTMLTextareaAttributes {
    autocorrect?: "on" | "off";
  }
}
