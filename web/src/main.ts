import { mount } from "svelte";
import App from "./App.svelte";
import "./styles.css";

const theme = matchMedia("(prefers-color-scheme: light)");
const applyTheme = () =>
  document.documentElement.classList.toggle("dark", !theme.matches);
applyTheme();
theme.addEventListener("change", applyTheme);
mount(App, { target: document.getElementById("app")! });
