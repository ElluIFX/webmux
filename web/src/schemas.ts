import { z } from "zod";

export const SessionInfoSchema = z.object({
  id: z.string().regex(/^[a-z2-7]{8}$/),
  title: z.string(),
  cwd: z.string(),
  preset: z.string(),
  cmd: z.array(z.string()).min(1),
  status: z.enum(["running", "waiting", "idle", "exited"]),
  exitCode: z.number().int().nullable(),
  lastLine: z.string(),
  createdAt: z.iso.datetime({ offset: true }),
  lastActiveAt: z.iso.datetime({ offset: true }),
  cols: z.number().int().min(1).max(1000),
  rows: z.number().int().min(1).max(1000),
  clients: z.number().int().nonnegative(),
});
export type SessionInfo = z.infer<typeof SessionInfoSchema>;

export const PresetsSchema = z.array(
  z.object({
    name: z.string(),
    label: z.string(),
    cmd: z.array(z.string()).min(1),
    allowArgs: z.boolean(),
  }),
);
export type Preset = z.infer<typeof PresetsSchema>[number];

export const SessionEventSchema = z.discriminatedUnion("type", [
  z.object({
    type: z.literal("snapshot"),
    sessions: z.array(SessionInfoSchema),
  }),
  z.object({ type: z.literal("created"), session: SessionInfoSchema }),
  z.object({ type: z.literal("updated"), session: SessionInfoSchema }),
  z.object({ type: z.literal("removed"), id: SessionInfoSchema.shape.id }),
]);
export const ExitSchema = z.object({ exitCode: z.number().int() });
export const HealthSchema = z.object({
  ok: z.literal(true),
  sessions: z.number().int().nonnegative(),
});
export const ConfigSchema = z.object({ defaultCwd: z.string() });
export const DirectoriesSchema = z.array(z.string());
export const ErrorSchema = z.object({ error: z.string() });
export const RecentDirsSchema = z
  .array(z.string())
  .transform((dirs) => dirs.slice(0, 10));
export const FontSizeSchema = z.coerce.number().int().min(12).max(20);
export const TextSettingSchema = z.string();
export const NoContentSchema = z.undefined();
