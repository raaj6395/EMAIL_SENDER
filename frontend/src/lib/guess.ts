// Best-effort guesses of company + first name from a recipient email address.
// These are only suggestions — the user can overwrite anything.

// Generic mailbox providers whose domain says nothing about a company.
const GENERIC_DOMAINS = new Set([
  "gmail.com",
  "googlemail.com",
  "outlook.com",
  "hotmail.com",
  "live.com",
  "yahoo.com",
  "ymail.com",
  "icloud.com",
  "me.com",
  "proton.me",
  "protonmail.com",
  "aol.com",
  "zoho.com",
  "gmx.com",
  "mail.com",
]);

// Role/functional mailboxes that are not a person's name.
const ROLE_LOCALS = new Set([
  "hiring",
  "careers",
  "career",
  "jobs",
  "job",
  "recruiting",
  "recruitment",
  "recruiter",
  "hr",
  "talent",
  "info",
  "contact",
  "hello",
  "team",
  "support",
  "admin",
  "office",
  "people",
  "work",
  "apply",
  "cv",
  "resume",
  "no-reply",
  "noreply",
]);

/** guessCompany derives a company name from the email domain, or "" if generic. */
export function guessCompany(email: string): string {
  const at = email.indexOf("@");
  if (at < 0) return "";
  const domain = email.slice(at + 1).trim().toLowerCase();
  if (!domain || !domain.includes(".")) return "";
  if (GENERIC_DOMAINS.has(domain)) return "";

  // Take the label just before the public suffix, e.g. "carousell" from
  // "carousell.com" or "jobs.carousell.co.uk".
  const parts = domain.split(".");
  // For common two-part TLDs (co.uk, com.sg), the company is the 3rd-from-last.
  const twoPartTLDs = new Set(["co", "com", "org", "net", "ac", "gov", "edu"]);
  let idx = parts.length - 2;
  if (parts.length >= 3 && twoPartTLDs.has(parts[parts.length - 2])) {
    idx = parts.length - 3;
  }
  const label = parts[Math.max(0, idx)];
  if (!label) return "";
  return capitalize(label.replace(/[-_]/g, " "));
}

/** guessFirstName derives a first name from the local part, or "" if it looks
 *  like a role mailbox / opaque handle. First name only. */
export function guessFirstName(email: string): string {
  const at = email.indexOf("@");
  if (at <= 0) return "";
  let local = email.slice(0, at).trim().toLowerCase();
  if (!local) return "";

  // Drop "+tag" suffixes (priya+jobs@…).
  local = local.split("+")[0];

  // The first token before a separator is the first name.
  const first = local.split(/[._-]/)[0];
  if (!first) return "";
  if (ROLE_LOCALS.has(first) || ROLE_LOCALS.has(local)) return "";

  // Reject tokens that don't look like a name: must be mostly letters and a
  // sensible length (avoids "px2847", "u12345").
  if (!/^[a-z]{2,20}$/.test(first)) return "";

  return capitalize(first);
}

function capitalize(s: string): string {
  return s
    .split(" ")
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
}
