import { onMount } from "svelte";

export function useViewport() {
  let height = $state(window.visualViewport?.height ?? window.innerHeight);
  onMount(() => {
    const viewport = window.visualViewport;
    const resize = () => {
      height = viewport?.height ?? window.innerHeight;
      window.scrollTo(0, 0);
    };
    viewport?.addEventListener("resize", resize);
    viewport?.addEventListener("scroll", resize);
    window.addEventListener("resize", resize);
    return () => {
      viewport?.removeEventListener("resize", resize);
      viewport?.removeEventListener("scroll", resize);
      window.removeEventListener("resize", resize);
    };
  });
  return {
    get height() {
      return height;
    },
  };
}
