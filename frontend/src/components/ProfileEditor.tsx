"use client";

import { useState } from "react";
import { Profile } from "@/lib/api";
import { Button, Card, Field, Input, Textarea } from "./ui";

export function ProfileEditor({
  profile,
  onChange,
  onParse,
  onSave,
  parsing,
  saving,
  saved,
}: {
  profile: Profile;
  onChange: (p: Profile) => void;
  onParse: () => void;
  onSave: () => void;
  parsing: boolean;
  saving: boolean;
  saved: boolean;
}) {
  const set = <K extends keyof Profile>(key: K, value: Profile[K]) =>
    onChange({ ...profile, [key]: value });

  const [skillDraft, setSkillDraft] = useState("");

  const addSkill = () => {
    const v = skillDraft.trim();
    if (v && !profile.skills.includes(v)) {
      set("skills", [...profile.skills, v]);
    }
    setSkillDraft("");
  };

  const removeSkill = (s: string) =>
    set("skills", profile.skills.filter((x) => x !== s));

  return (
    <Card title="Your profile" collapsible defaultOpen={false}>
      <div className="mb-4 flex flex-wrap items-center gap-3">
        <Button variant="secondary" onClick={onParse} loading={parsing}>
          Parse from resume
        </Button>
        <span className="text-xs text-[var(--muted)]">
          Auto-fills fields from backend/data/resume.pdf. Review and correct before sending.
        </span>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Full name">
          <Input value={profile.name} onChange={(e) => set("name", e.target.value)} placeholder="Ankit Raj" />
        </Field>
        <Field label="Target role">
          <Input value={profile.targetRole} onChange={(e) => set("targetRole", e.target.value)} placeholder="Backend Engineer" />
        </Field>
        <Field label="Your email" hint="Shown in your signature">
          <Input value={profile.email} onChange={(e) => set("email", e.target.value)} placeholder="you@gmail.com" />
        </Field>
        <Field label="Phone (optional)">
          <Input value={profile.phone} onChange={(e) => set("phone", e.target.value)} placeholder="+65 1234 5678" />
        </Field>
      </div>

      <div className="mt-4">
        <Field label="Skills" hint="Type a skill and press Enter">
          <div className="flex flex-wrap gap-2 rounded-lg border border-[var(--border)] bg-[var(--background)] p-2">
            {profile.skills.map((s) => (
              <span
                key={s}
                className="inline-flex items-center gap-1 rounded-full bg-[var(--accent)]/10 px-2.5 py-1 text-xs font-medium text-[var(--accent)]"
              >
                {s}
                <button onClick={() => removeSkill(s)} className="opacity-60 hover:opacity-100" aria-label={`Remove ${s}`}>
                  ✕
                </button>
              </span>
            ))}
            <input
              value={skillDraft}
              onChange={(e) => setSkillDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === ",") {
                  e.preventDefault();
                  addSkill();
                } else if (e.key === "Backspace" && !skillDraft && profile.skills.length) {
                  removeSkill(profile.skills[profile.skills.length - 1]);
                }
              }}
              onBlur={addSkill}
              placeholder={profile.skills.length ? "" : "Go, Kubernetes, PostgreSQL…"}
              className="min-w-[120px] flex-1 bg-transparent px-1 py-1 text-sm outline-none"
            />
          </div>
        </Field>
      </div>

      <div className="mt-4">
        <Field label="Pitch" hint="One or two lines on what you bring — used in the email hook">
          <Textarea
            rows={2}
            value={profile.pitch}
            onChange={(e) => set("pitch", e.target.value)}
            placeholder="I build reliable, high-throughput backend systems…"
          />
        </Field>
      </div>

      <div className="mt-4 grid gap-4 sm:grid-cols-3">
        <Field label="LinkedIn">
          <Input value={profile.linkedin} onChange={(e) => set("linkedin", e.target.value)} placeholder="https://linkedin.com/in/…" />
        </Field>
        <Field label="GitHub">
          <Input value={profile.github} onChange={(e) => set("github", e.target.value)} placeholder="https://github.com/…" />
        </Field>
        <Field label="Portfolio">
          <Input value={profile.portfolio} onChange={(e) => set("portfolio", e.target.value)} placeholder="https://…" />
        </Field>
      </div>

      <div className="mt-5 flex items-center gap-3">
        <Button onClick={onSave} loading={saving}>
          Save profile
        </Button>
        {saved && <span className="text-xs text-green-600 dark:text-green-400">✓ Saved</span>}
      </div>
    </Card>
  );
}
