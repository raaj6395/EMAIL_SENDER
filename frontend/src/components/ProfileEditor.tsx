"use client";

import { useState } from "react";
import { Profile } from "@/lib/api";
import { Button, Card, Field, Input, Textarea } from "./ui";
import { IconCheck, IconX } from "./icons";

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
      <div className="mb-6 flex flex-wrap items-center gap-3">
        <Button variant="secondary" onClick={onParse} loading={parsing}>
          Fill from resume
        </Button>
        <span className="text-[12px] text-[var(--subtle)]">
          Reads the resume for the selected profile. Check every field before sending
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

      <div className="mt-5">
        <Field label="Skills" hint="Type a skill and press Enter">
          <div className="flex min-h-9 flex-wrap items-center gap-1.5 rounded-[var(--radius-md)] border border-[var(--border-strong)] bg-[var(--surface)] p-1.5 focus-within:ring-2 focus-within:ring-[var(--ring)]">
            {profile.skills.map((s) => (
              <span
                key={s}
                className="inline-flex h-6 items-center gap-1 rounded-[4px] border border-[var(--border)] bg-[var(--elevated)] pl-2 pr-1 text-[13px] text-[var(--fg)]"
              >
                {s}
                <button onClick={() => removeSkill(s)} className="rounded-[3px] text-[var(--subtle)] transition-colors hover:text-[var(--fg)]" aria-label={`Remove ${s}`}>
                  <IconX size={14} />
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
              placeholder={profile.skills.length ? "" : "Go, Kubernetes, PostgreSQL"}
              className="min-w-32 flex-1 bg-transparent px-1.5 text-[14px] outline-none placeholder:text-[var(--subtle)]"
            />
          </div>
        </Field>
      </div>

      <div className="mt-5">
        <Field label="Pitch" hint="One or two lines on what you bring. Used as the opening hook">
          <Textarea
            rows={2}
            value={profile.pitch}
            onChange={(e) => set("pitch", e.target.value)}
            placeholder="I build reliable, high-throughput backend systems"
          />
        </Field>
      </div>

      <div className="mt-5 grid gap-4 sm:grid-cols-3">
        <Field label="LinkedIn">
          <Input value={profile.linkedin} onChange={(e) => set("linkedin", e.target.value)} placeholder="linkedin.com/in/username" />
        </Field>
        <Field label="GitHub">
          <Input value={profile.github} onChange={(e) => set("github", e.target.value)} placeholder="github.com/username" />
        </Field>
        <Field label="Portfolio">
          <Input value={profile.portfolio} onChange={(e) => set("portfolio", e.target.value)} placeholder="yoursite.dev" />
        </Field>
      </div>

      <div className="mt-6 flex items-center gap-3">
        <Button onClick={onSave} loading={saving}>
          Save profile
        </Button>
        {saved && (
          <span className="inline-flex items-center gap-1.5 text-[13px] text-[var(--success-fg)]">
            <IconCheck size={14} />
            Saved
          </span>
        )}
      </div>
    </Card>
  );
}
