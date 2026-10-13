import { useEffect, useState } from "preact/hooks";
export function useViewport() {
  const [height, setHeight] = useState(
    window.visualViewport?.height ?? window.innerHeight,
  );
  useEffect(() => {
    const viewport = window.visualViewport;
    const resize = () => {
      setHeight(viewport?.height ?? window.innerHeight);
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
  }, []);
  return height;
}
