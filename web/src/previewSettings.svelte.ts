import { stored, store } from "./api";
import { PreviewSettingsSchema, type PreviewSettings } from "./schemas";

export const defaultPreviewSettings: PreviewSettings = { lines: 10, skip: 1 };

function savedSettings(): PreviewSettings {
  try {
    return PreviewSettingsSchema.parse(
      JSON.parse(stored("preview", JSON.stringify(defaultPreviewSettings))),
    );
  } catch {
    return { ...defaultPreviewSettings };
  }
}

export const previewSettings = $state(savedSettings());

export function updatePreviewSettings(value: PreviewSettings) {
  const settings = PreviewSettingsSchema.parse(value);
  Object.assign(previewSettings, settings);
  store("preview", JSON.stringify(settings));
}
