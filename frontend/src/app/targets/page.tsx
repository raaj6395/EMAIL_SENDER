"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ApiError,
  LPA_FLOORS,
  TARGET_CATEGORIES,
  TargetCategory,
  TargetCompany,
  api,
  setComposePrefill,
  targetCategoryLabel,
} from "@/lib/api";
import { Alerts, Badge, Button, buttonClass, Card, EmptyState, Field, Input, Notice, Page, SectionHeader, Select, Spinner, Toast } from "@/components/ui";
import { ContactRow, RowList } from "@/components/hr";
import { IconBuilding, IconPin } from "@/components/icons";

/** One company row: comp band, category, note, and the outreach actions. */
function CompanyRow({
  c,
  onCompose,
  onRemove,
}: {
  c: TargetCompany;
  onCompose: (c: TargetCompany) => void;
  onRemove: (c: TargetCompany) => void;
}) {
  const band = c.maxLPA > c.minLPA ? `${c.minLPA}–${c.maxLPA}` : `${c.minLPA}`;

  return (
    <ContactRow
      actions={
        <>
          {c.careersUrl && (
            <a href={c.careersUrl} target="_blank" rel="noopener noreferrer" className={buttonClass("ghost", "sm")}>
              Careers
            </a>
          )}
          <Button variant="ghost" size="sm" onClick={() => onRemove(c)}>
            Remove
          </Button>
          <Button size="sm" onClick={() => onCompose(c)}>
            Compose
          </Button>
        </>
      }
    >
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <span className="text-[14px] font-medium">{c.name}</span>
        <span className="font-mono text-[13px] text-[var(--fg)]">₹{band} LPA</span>
        <span className="text-[13px] text-[var(--muted)]">{targetCategoryLabel(c.category)}</span>
        {c.confidence === "low" && <Badge tone="warning">Low confidence</Badge>}
        {c.source === "manual" && <Badge>Added by you</Badge>}
      </div>
      {c.note && <p className="mt-1 text-[13px] leading-5 text-[var(--muted)]">{c.note}</p>}
      <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-[12px] text-[var(--subtle)]">
        {c.hq && (
          <span className="inline-flex items-center gap-1">
            <IconPin size={14} />
            {c.hq}
          </span>
        )}
        {c.emailDomain && <span className="font-mono">@{c.emailDomain}</span>}
        {c.tech?.length > 0 && <span>{c.tech.slice(0, 4).join(" · ")}</span>}
      </div>
    </ContactRow>
  );
}

export default function TargetsPage() {
  const router = useRouter();

  const [companies, setCompanies] = useState<TargetCompany[]>([]);
  const [cached, setCached] = useState(0);
  const [aiEnabled, setAiEnabled] = useState(true);
  const [loading, setLoading] = useState(true);

  const [minLPA, setMinLPA] = useState(12);
  const [cats, setCats] = useState<TargetCategory[]>([]);
  const [q, setQ] = useState("");

  const [discovering, setDiscovering] = useState(false);
  const [addName, setAddName] = useState("");
  const [adding, setAdding] = useState(false);
  const [toast, setToast] = useState<{ kind: "success" | "error" | "info"; message: string } | null>(
    null
  );

  const errMsg = (e: unknown) =>
    e instanceof ApiError ? e.message : "Something went wrong. Please try again.";

  const load = useCallback(
    async (opts: { minLPA: number; cats: TargetCategory[]; q: string }) => {
      try {
        const res = await api.targets({
          minLPA: opts.minLPA,
          categories: opts.cats,
          q: opts.q,
        });
        setCompanies(res.companies);
        setCached(res.cached);
        setAiEnabled(res.aiEnabled);
      } catch (e) {
        setToast({ kind: "error", message: errMsg(e) });
      } finally {
        setLoading(false);
      }
    },
    []
  );

  // Reload whenever a filter changes (debounced for the text query).
  useEffect(() => {
    const id = setTimeout(() => {
      load({ minLPA, cats, q });
    }, 250);
    return () => clearTimeout(id);
  }, [load, minLPA, cats, q]);

  const toggleCat = (c: TargetCategory) =>
    setCats((prev) => (prev.includes(c) ? prev.filter((x) => x !== c) : [...prev, c]));

  const handleDiscover = async () => {
    setDiscovering(true);
    setToast(null);
    try {
      const res = await api.targetsDiscover({ minLPA, categories: cats, count: 25 });
      if (res.count === 0) {
        setToast({
          kind: "info",
          message: "No new companies this time. Try a different pay floor or category",
        });
      } else {
        setToast({
          kind: "success",
          message: `Found ${res.count} new ${res.count === 1 ? "company" : "companies"}: ${res.added
            .slice(0, 6)
            .map((c) => c.name)
            .join(", ")}${res.count > 6 ? "…" : ""}`,
        });
      }
      await load({ minLPA, cats, q });
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setDiscovering(false);
    }
  };

  const handleAdd = async () => {
    const name = addName.trim();
    if (!name) return;
    setAdding(true);
    setToast(null);
    try {
      const c = await api.targetsAdd(name);
      setAddName("");
      setToast({
        kind: "success",
        message: `${c.name}: ₹${c.minLPA}–${c.maxLPA} LPA (${targetCategoryLabel(c.category)})`,
      });
      await load({ minLPA, cats, q });
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setAdding(false);
    }
  };

  const handleRemove = async (c: TargetCompany) => {
    try {
      await api.targetsRemove(c.name);
      setCompanies((prev) => prev.filter((x) => x.name !== c.name));
      setCached((n) => Math.max(0, n - 1));
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    }
  };

  // Jump to the compose form with this company pre-filled. There's no HR email
  // address here (this is a company list, not a contact list), so the user picks
  // the recipient — the domain hint helps them guess or look one up.
  const handleCompose = (c: TargetCompany) => {
    setComposePrefill({
      input: { recipientEmail: "", recipientName: "", company: c.name },
    });
    router.push("/");
  };

  return (
    <Page>
      <SectionHeader
        title="Target companies"
        subtitle="Hypergrowth startups, product MNCs and GCCs that pay freshers above your CTC floor. Built by AI from company knowledge rather than job boards, since Indian postings rarely publish salary"
        action={<span className="font-mono text-[12px] text-[var(--subtle)]">{cached} saved</span>}
      />

      <Alerts>
        {toast && <Toast kind={toast.kind} message={toast.message} onClose={() => setToast(null)} />}
        {!aiEnabled && (
          <Notice tone="warning">
            Set <span className="font-mono">OPENAI_API_KEY</span> in <span className="font-mono">backend/.env</span> to build
            and grow this list
          </Notice>
        )}
      </Alerts>

      <Card title="Filters" className="mb-8">
        <div className="flex flex-wrap items-end gap-4">
          <Field label="Pays at least">
            <Select
              value={minLPA}
              onChange={(e) => setMinLPA(Number(e.target.value))}
              className="w-36"
            >
              {LPA_FLOORS.map((n) => (
                <option key={n} value={n}>
                  ₹{n}+ LPA
                </option>
              ))}
            </Select>
          </Field>

          <div className="min-w-52 flex-1">
            <Field label="Search">
              <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Name, tech or city" />
            </Field>
          </div>
        </div>

        <div className="mt-4 flex flex-wrap gap-2">
          {TARGET_CATEGORIES.map((c) => {
            const on = cats.includes(c.value);
            return (
              <button
                key={c.value}
                onClick={() => toggleCat(c.value)}
                aria-pressed={on}
                className={`h-8 rounded-[var(--radius-md)] border px-3 text-[13px] transition-colors duration-150 ${
                  on
                    ? "border-[var(--primary)] bg-[var(--primary)] text-[var(--primary-fg)]"
                    : "border-[var(--border-strong)] text-[var(--muted)] hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]"
                }`}
              >
                {c.label}
              </button>
            );
          })}
          {cats.length > 0 && (
            <button
              onClick={() => setCats([])}
              className="h-8 px-2 text-[13px] text-[var(--subtle)] underline underline-offset-2 hover:text-[var(--fg)]"
            >
              Clear
            </button>
          )}
        </div>

        <div className="mt-5 flex flex-wrap items-center gap-3 border-t border-[var(--border)] pt-5">
          <Button onClick={handleDiscover} loading={discovering} disabled={!aiEnabled}>
            {discovering ? "Finding companies" : "Find more companies"}
          </Button>
          <span className="text-[12px] text-[var(--subtle)]">Asks for 25 more that match your filters, skipping ones you have</span>
        </div>

        <div className="mt-5 flex flex-wrap items-end gap-2">
          <div className="min-w-52 flex-1">
            <Field label="Add a specific company">
            <Input
              value={addName}
              onChange={(e) => setAddName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleAdd();
              }}
              placeholder="Sprinklr"
              disabled={!aiEnabled}
            />
            </Field>
          </div>
          <Button variant="secondary" onClick={handleAdd} loading={adding} disabled={!addName.trim() || !aiEnabled}>
            Add
          </Button>
        </div>
      </Card>

      {loading ? (
        <div className="flex justify-center py-10">
          <Spinner />
        </div>
      ) : companies.length === 0 ? (
        <EmptyState icon={<IconBuilding size={20} />} title={cached === 0 ? "No companies yet" : "No matches for these filters"}>
          {cached === 0 ? "Find more companies builds your target list" : "Try a lower CTC floor, or clear the categories"}
        </EmptyState>
      ) : (
        <section>
          <p className="mb-3 text-[13px] text-[var(--muted)]">
            {companies.length} {companies.length === 1 ? "company" : "companies"}, best paying first
          </p>
          <RowList>
            {companies.map((c) => (
              <CompanyRow key={c.name} c={c} onCompose={handleCompose} onRemove={handleRemove} />
            ))}
          </RowList>
        </section>
      )}

      <p className="mt-12 border-t border-[var(--border)] pt-6 text-[12px] leading-4 text-[var(--subtle)]">
        Pay figures are AI estimates of typical fresher CTC, not live market data or offers. Use them to decide who to
        target, and check Levels.fyi or AmbitionBox before deciding anything.
      </p>
    </Page>
  );
}
