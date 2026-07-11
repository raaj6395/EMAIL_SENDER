# Resume Cold-Email Sender

A local web app to send a tailored resume email to any company. Type a recipient
email + company name, preview a personalized email (built from your resume), and
send it through your **Gmail** — with your resume attached automatically.

- **Backend:** Go (`net/http`, [go-mail](https://github.com/wneessen/go-mail) for SMTP, [ledongthuc/pdf](https://github.com/ledongthuc/pdf) for resume parsing)
- **Frontend:** Next.js + React + Tailwind
- **Email:** Gmail SMTP. Emails are **written by AI** (OpenAI, tailored per company) when an OpenAI key is configured, and **automatically fall back to a built-in template** if the key is missing or the API fails — so sending never breaks.
- **Digest:** an on-demand button emails a summary of all your sends to a configured address.

---

## How it works

1. Drop your resume PDF on the backend once.
2. Your profile comes **pre-filled** on first run (name, email, phone, skills, links).
   Adjust anything in the Profile section, or click **Parse from resume** to
   re-extract from the PDF, then **Save profile**.
3. Enter a recipient email + **recipient name** (optional — becomes “Hi {name},”)
   + company name → **Preview** the exact email → **Send**.
4. Every send is logged in the **Track** section (send history).

The email is written in a warm, natural, first-person style (matching a real
job-application voice) — as flowing paragraphs, not bullet points.

Your resume and Gmail password never leave the backend — the browser only ever
sees rendered previews.

---

## Prerequisites

- **Go** 1.22+ (uses the pattern-based `net/http` mux)
- **Node.js** 18+ and npm
- A **Gmail account** with 2-Step Verification enabled

---

## 1. Get a Gmail App Password

Gmail SMTP will **not** accept your normal password. You need a 16-character App Password:

1. Enable 2-Step Verification: <https://myaccount.google.com/security>
2. Create an App Password: <https://myaccount.google.com/apppasswords>
   - Name it anything (e.g. "resume-sender"). Google shows a 16-char password.
3. Copy it (spaces don't matter — `abcd efgh ijkl mnop` works as `abcdefghijklmnop`).

> If you don't see "App passwords", make sure 2-Step Verification is **on** first.

---

## 2. Configure & run the backend

```bash
cd backend

# Create your .env from the template
cp .env.example .env
#   → edit .env and set:
#       GMAIL_USER=you@gmail.com
#       GMAIL_APP_PASSWORD=abcdefghijklmnop

# Put your resume here (exact path/name):
#   backend/data/resume.pdf

go mod tidy
go run .
```

Backend starts on **http://localhost:8080**. On startup it logs whether it found
your resume and credentials:

```
email-sender backend listening on :8080
  resume found: true
  gmail creds:  true
```

---

## 3. Run the frontend

In a second terminal:

```bash
cd frontend
npm install
npm run dev
```

Open **http://localhost:3000**.

The frontend talks to the backend at `http://localhost:8080` (configurable via
`frontend/.env.local` → `NEXT_PUBLIC_API_URL`).

---

## 4. First send (test it on yourself)

Before emailing real companies, send one to **your own email address**:

1. Parse resume → review fields → Save profile.
2. Recipient = your own email, Company = anything.
3. Preview → Send.
4. Check your inbox: correct subject, formatted body, and `YourName_Resume.pdf` attached.

Once that looks right, you're ready to send to real recipients.

---

## Project layout

```
EMAIL_SENDER/
├── backend/
│   ├── main.go                     # HTTP server, routes, CORS, handlers
│   ├── internal/
│   │   ├── config/config.go        # .env loading + validation
│   │   ├── resume/parse.go         # PDF → text → profile extraction
│   │   ├── resume/profile.go       # profile model + JSON persistence
│   │   ├── email/template.go       # subject + HTML/plaintext body rendering
│   │   ├── email/send.go           # go-mail SMTP send with attachment
│   │   └── email/history.go        # send log (JSON)
│   ├── data/                       # resume.pdf, profile.json, history.json (gitignored)
│   └── .env                        # your Gmail creds (gitignored)
└── frontend/
    └── src/
        ├── app/page.tsx            # the stepped flow
        ├── lib/api.ts              # typed backend client
        └── components/             # ProfileEditor, ComposeForm, EmailPreview, …
```

---

## API reference (backend)

| Method | Path | Purpose |
|--------|------|---------|
| `GET`  | `/api/health` | Server status; whether resume + creds are present |
| `POST` | `/api/parse-resume` | Parse `data/resume.pdf` into an editable profile |
| `GET`  | `/api/profile` | Read the saved profile |
| `PUT`  | `/api/profile` | Save edited profile |
| `POST` | `/api/preview` | `{recipientEmail, company, role?}` → rendered email |
| `POST` | `/api/send` | Same input → sends via Gmail, logs history |
| `GET`  | `/api/history` | List of past sends |
| `POST` | `/api/digest` | Emails a summary of all sends to `DIGEST_TO` |

---

## AI emails & digest (optional)

Add these to `backend/.env`:

```bash
OPENAI_API_KEY=sk-...        # enables AI-written emails
OPENAI_MODEL=gpt-4o          # default; gpt-4o-mini is cheaper
DIGEST_TO=you@gmail.com      # where the "Email digest" button sends
```

- **AI on:** the header shows an "✨ AI on" badge; each preview shows whether it
  was written by AI or the template. If OpenAI fails (bad key, quota, timeout),
  the preview silently falls back to the template and tells you why.
- **AI off:** just leave `OPENAI_API_KEY` unset — everything works template-only.
- **Digest:** click **Email digest** in the Track section to send a summary of
  all your sends (company, recipient, status, time) to `DIGEST_TO`.

## Can it tell if someone replied?

**No — not in this version, and it's an honest limitation, not an oversight.**

This app only **sends** mail (outbound SMTP). Knowing whether a recipient
*replied* requires **reading your inbox**, which is a completely different
capability:

- **Gmail API or IMAP** access to your inbox (OAuth or IMAP app password), then
  matching incoming messages by thread / `In-Reply-To` headers to your sends.
- Open/click tracking (a tracking pixel) can hint at *opens*, but it's unreliable
  (blocked by Gmail image proxying) and hurts deliverability — not recommended.

So today, "did they reply?" is answered by **checking your Gmail inbox directly**.
If you want automated reply detection built in, that's a follow-up feature — say
the word and it can be added via the Gmail API (read-only inbox scope).

## Notes & limits

- **Gmail sending limits:** ~500 emails/day for personal accounts. This tool is
  built for targeted, manual sends — not bulk blasting (which hurts deliverability
  and can get your account flagged).
- **Deliverability:** cold emails can land in spam. The template keeps it short,
  specific, and value-first to maximize replies, but a warm intro always beats a
  cold email.
- **Storage:** flat JSON files in `backend/data/` (single-user local tool). No database.
- **Security:** the API has no auth and is intended to run locally only. Don't
  expose port 8080 to the internet.
