"use client";

import { useCallback, useEffect, useState } from "react";
import {
  ApiError,
  SendSchedule as Schedule,
  SendWindow,
  WEEKDAY_NAMES,
  api,
  formatCountdown,
  hhmmToMin,
  minToHHMM,
} from "@/lib/api";
import { Button, Card, Dot, EmptyRow, Input, Notice, Select } from "./ui";
import { IconCheck, IconPlus, IconX } from "./icons";

/** A sensible new row when the user clicks "Add window". */
const newWindow = (): SendWindow => ({ day: 1, startMin: 11 * 60, endMin: 13 * 60 });

/** One editable window row: day + start/end time. */
function WindowRow({
  window: w,
  onChange,
  onRemove,
  disabled,
}: {
  window: SendWindow;
  onChange: (w: SendWindow) => void;
  onRemove: () => void;
  disabled: boolean;
}) {
  const invalid = w.endMin <= w.startMin;

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Select
        value={w.day}
        disabled={disabled}
        onChange={(e) => onChange({ ...w, day: Number(e.target.value) })}
        className="w-36"
        aria-label="Day of week"
      >
        {WEEKDAY_NAMES.map((name, i) => (
          <option key={name} value={i}>
            {name}
          </option>
        ))}
      </Select>

      <Input
        type="time"
        value={minToHHMM(w.startMin)}
        disabled={disabled}
        onChange={(e) => onChange({ ...w, startMin: hhmmToMin(e.target.value) })}
        className="w-32"
        aria-label="Window start"
      />
      <span className="text-[13px] text-[var(--subtle)]">to</span>
      <Input
        type="time"
        value={minToHHMM(w.endMin)}
        disabled={disabled}
        onChange={(e) => onChange({ ...w, endMin: hhmmToMin(e.target.value) })}
        className="w-32"
        aria-label="Window end"
      />

      <Button variant="ghost" size="sm" onClick={onRemove} disabled={disabled} aria-label="Remove window" className="w-8 px-0">
        <IconX />
      </Button>

      {invalid && <span className="text-[12px] text-[var(--danger-fg)]">End must be after start</span>}
    </div>
  );
}

/**
 * Editor for the weekly send windows. Queued bulk email only goes out inside
 * these windows, so outreach lands while HR is actually at their desk.
 */
export function SendScheduleCard({ onSaved }: { onSaved?: () => void }) {
  const [server, setServer] = useState<Schedule | null>(null);
  const [enabled, setEnabled] = useState(true);
  const [windows, setWindows] = useState<SendWindow[]>([]);
  const [cap, setCap] = useState(40);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [savedAt, setSavedAt] = useState(false);

  const applyServer = useCallback((s: Schedule) => {
    setServer(s);
    setEnabled(s.enabled);
    setWindows(s.windows);
    setCap(s.perWindowCap);
  }, []);

  // Load on mount, then refresh every minute to keep the "opens in" countdown
  // honest. Both paths update state from the fetch callback, never synchronously
  // in the effect body.
  useEffect(() => {
    let alive = true;
    const fetchOnce = () => {
      api
        .schedule()
        .then((s) => {
          if (alive) applyServer(s);
        })
        .catch(() => {
          /* the card just stays empty if the backend is down */
        });
    };
    fetchOnce();
    const id = setInterval(fetchOnce, 60_000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [applyServer]);

  const dirty =
    !!server &&
    (enabled !== server.enabled ||
      cap !== server.perWindowCap ||
      JSON.stringify(windows.map((w) => [w.day, w.startMin, w.endMin])) !==
        JSON.stringify(server.windows.map((w) => [w.day, w.startMin, w.endMin])));

  const hasInvalid = windows.some((w) => w.endMin <= w.startMin);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      applyServer(await api.saveSchedule({ enabled, perWindowCap: cap, windows }));
      setSavedAt(true);
      setTimeout(() => setSavedAt(false), 2500);
      onSaved?.();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not save the schedule.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card
      title="Send schedule"
      collapsible
      defaultOpen={false}
      action={
        server ? (
          server.enabled ? (
            server.open ? (
              <span className="inline-flex items-center gap-2 text-[12px] text-[var(--success-fg)]">
                <Dot tone="success" />
                Window open
              </span>
            ) : (
              <span className="font-mono text-[12px] text-[var(--subtle)]">opens in {formatCountdown(server.opensInSec)}</span>
            )
          ) : (
            <span className="text-[12px] text-[var(--warning-fg)]">Off</span>
          )
        ) : null
      }
    >
      <p className="mb-5 text-[13px] leading-5 text-[var(--muted)]">
        Queued emails only go out inside these windows, so they land while HR is at their desk instead of
        overnight. Anything queued outside a window waits for the next one.
        {server?.timezone && (
          <>
            {" "}
            Times are in <span className="text-[var(--fg)]">{server.timezone}</span>, server time.
          </>
        )}
      </p>

      <label className="mb-5 flex items-center gap-2.5 text-[14px]">
        <input
          type="checkbox"
          checked={enabled}
          onChange={(e) => setEnabled(e.target.checked)}
          className="h-4 w-4 accent-[var(--accent)]"
        />
        <span>
          Only send during these windows
          <span className="ml-1 text-[var(--subtle)]">Off sends queued email at any hour</span>
        </span>
      </label>

      <div className="space-y-2">
        {windows.map((w, i) => (
          <WindowRow
            key={i}
            window={w}
            disabled={!enabled}
            onChange={(next) => setWindows((ws) => ws.map((x, j) => (j === i ? next : x)))}
            onRemove={() => setWindows((ws) => ws.filter((_, j) => j !== i))}
          />
        ))}
        {windows.length === 0 && (
          <EmptyRow>No windows yet. Add one, or turn the schedule off to send right away</EmptyRow>
        )}
      </div>

      <div className="mt-3">
        <Button variant="secondary" size="sm" onClick={() => setWindows((ws) => [...ws, newWindow()])} disabled={!enabled}>
          <IconPlus />
          Add window
        </Button>
      </div>

      <div className="mt-5 flex flex-wrap items-end gap-4 border-t border-[var(--border)] pt-5">
        <label className="block">
          <span className="mb-2 block text-[13px] font-medium">Max emails per window</span>
          <Input
            type="number"
            min={0}
            value={cap}
            onChange={(e) => setCap(Math.max(0, Number(e.target.value) || 0))}
            className="w-28"
          />
        </label>
        <p className="mb-2 min-w-48 flex-1 text-[12px] leading-4 text-[var(--subtle)]">
          Leftovers roll over to the next window. <span className="font-mono">0</span> means no limit, but a cap
          keeps Gmail from seeing a burst of cold email
        </p>
      </div>

      {error && (
        <Notice tone="danger" className="mt-3">
          {error}
        </Notice>
      )}

      <div className="mt-5 flex items-center gap-3">
        <Button onClick={handleSave} loading={saving} disabled={!dirty || hasInvalid}>
          Save schedule
        </Button>
        {savedAt && (
          <span className="inline-flex items-center gap-1.5 text-[13px] text-[var(--success-fg)]">
            <IconCheck size={14} />
            Saved
          </span>
        )}
        {dirty && !savedAt && <span className="text-[12px] text-[var(--subtle)]">Unsaved changes</span>}
      </div>
    </Card>
  );
}
