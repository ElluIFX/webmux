export interface TerminalDraft {
  readonly sessionId: string;
  readonly identity: symbol;
  enabled: boolean;
  text: string;
  selectionStart: number | null;
  selectionEnd: number | null;
  draftEpoch: number;
  updatedWhileHidden: boolean;
}

export interface DraftOperation {
  readonly sessionId: string;
  readonly entryIdentity: symbol;
  readonly draftEpoch: number;
  readonly startedInDraft: boolean;
}

export type InputDestination = "draft" | "terminal" | "failed";
export type TextInsertion = InputDestination;

// Owned by App, never persisted. An operation can only use its original entry.
export function createTerminalDrafts(): TerminalDrafts {
  const entries = new Map<string, TerminalDraft>();
  function get(sessionId: string): TerminalDraft {
    let entry = entries.get(sessionId);
    if (!entry) {
      const created = $state<TerminalDraft>({
        sessionId,
        identity: Symbol(sessionId),
        enabled: false,
        text: "",
        selectionStart: null,
        selectionEnd: null,
        draftEpoch: 0,
        updatedWhileHidden: false,
      });
      entry = created;
      entries.set(sessionId, entry);
    }
    return entry;
  }
  function valid(entry: TerminalDraft) {
    return entries.get(entry.sessionId) === entry;
  }
  function begin(entry: TerminalDraft): DraftOperation {
    return {
      sessionId: entry.sessionId,
      entryIdentity: entry.identity,
      draftEpoch: entry.draftEpoch,
      startedInDraft: entry.enabled,
    };
  }
  function destination(
    entry: TerminalDraft,
    operation = begin(entry),
  ): InputDestination {
    if (
      !valid(entry) ||
      operation.sessionId !== entry.sessionId ||
      operation.entryIdentity !== entry.identity
    )
      return "failed";
    return entry.enabled ||
      operation.startedInDraft ||
      entry.draftEpoch > operation.draftEpoch
      ? "draft"
      : "terminal";
  }
  function select(
    entry: TerminalDraft,
    start: number | null,
    end: number | null,
  ) {
    if (!valid(entry)) return;
    entry.selectionStart =
      start === null ? null : Math.max(0, Math.min(entry.text.length, start));
    entry.selectionEnd =
      end === null
        ? null
        : Math.max(entry.selectionStart ?? 0, Math.min(entry.text.length, end));
  }
  function edit(
    entry: TerminalDraft,
    text: string,
    start: number | null = null,
    end: number | null = start,
  ) {
    if (!valid(entry)) return;
    entry.text = text;
    select(entry, start, end);
  }
  function insert(
    entry: TerminalDraft,
    text: string,
    operation = begin(entry),
    submit?: (text: string) => boolean,
  ): TextInsertion {
    const target = destination(entry, operation);
    if (target === "terminal") return submit?.(text) ? "terminal" : "failed";
    if (target === "failed") return target;
    const start = entry.selectionStart ?? entry.text.length;
    const end = entry.selectionEnd ?? start;
    edit(
      entry,
      entry.text.slice(0, start) + text + entry.text.slice(end),
      start + text.length,
    );
    if (!entry.enabled) entry.updatedWhileHidden = true;
    return "draft";
  }
  return {
    get,
    valid,
    begin,
    destination,
    select,
    edit,
    insert,
    setEnabled(entry: TerminalDraft, enabled: boolean) {
      if (!valid(entry)) return;
      if (enabled && !entry.enabled) entry.draftEpoch++;
      entry.enabled = enabled;
      if (enabled) entry.updatedWhileHidden = false;
    },
    clear(entry: TerminalDraft) {
      edit(entry, "", 0);
      if (valid(entry)) entry.updatedWhileHidden = false;
    },
    remove(sessionId: string) {
      const entry = entries.get(sessionId);
      if (entry) {
        edit(entry, "", null);
        entry.enabled = false;
        entry.updatedWhileHidden = false;
      }
      entries.delete(sessionId);
    },
    reconcile(sessionIds: Iterable<string>) {
      const present = new Set(sessionIds);
      for (const id of entries.keys()) if (!present.has(id)) this.remove(id);
    },
    clearAll() {
      for (const id of entries.keys()) this.remove(id);
    },
  };
}

export interface TerminalDrafts {
  get(sessionId: string): TerminalDraft;
  valid(entry: TerminalDraft): boolean;
  begin(entry: TerminalDraft): DraftOperation;
  destination(
    entry: TerminalDraft,
    operation?: DraftOperation,
  ): InputDestination;
  select(entry: TerminalDraft, start: number | null, end: number | null): void;
  edit(
    entry: TerminalDraft,
    text: string,
    start?: number | null,
    end?: number | null,
  ): void;
  insert(
    entry: TerminalDraft,
    text: string,
    operation?: DraftOperation,
    submit?: (text: string) => boolean,
  ): TextInsertion;
  setEnabled(entry: TerminalDraft, enabled: boolean): void;
  clear(entry: TerminalDraft): void;
  remove(sessionId: string): void;
  reconcile(sessionIds: Iterable<string>): void;
  clearAll(): void;
}
