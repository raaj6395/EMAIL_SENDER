// Line icons in the lucide style: 24-unit grid, 1.5 stroke, currentColor.
// Inlined so the app doesn't need an icon dependency for ~25 glyphs.

type P = { size?: number; className?: string };

function svg(children: React.ReactNode, { size = 16, className = "" }: P) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={`shrink-0 ${className}`}
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}

export const IconMail = (p: P) => svg(<><rect x="2.5" y="4.5" width="19" height="15" rx="2" /><path d="m3 7 9 6 9-6" /></>, p);
export const IconMailCheck = (p: P) => svg(<><path d="M21.5 12V6.5a2 2 0 0 0-2-2h-15a2 2 0 0 0-2 2v11a2 2 0 0 0 2 2H12" /><path d="m3 7 9 6 9-6" /><path d="m16 19 2 2 4-4" /></>, p);
export const IconInbox = (p: P) => svg(<><path d="M21.5 12.5h-5.5l-2 3h-4l-2-3H2.5" /><path d="M5.5 5.1 2.5 12v6a2 2 0 0 0 2 2h15a2 2 0 0 0 2-2v-6l-3-6.9A2 2 0 0 0 16.7 4H7.3a2 2 0 0 0-1.8 1.1z" /></>, p);
export const IconBriefcase = (p: P) => svg(<><rect x="2.5" y="7" width="19" height="13" rx="2" /><path d="M16 7V5.5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2V7" /><path d="M2.5 13h19" /></>, p);
export const IconBuilding = (p: P) => svg(<><rect x="4" y="2.5" width="16" height="19" rx="1.5" /><path d="M9 21.5v-4h6v4M8.5 7h1M14.5 7h1M8.5 11h1M14.5 11h1" /></>, p);
export const IconUsers = (p: P) => svg(<><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" /></>, p);
export const IconSend = (p: P) => svg(<><path d="M21.5 2.5 10.5 13.5" /><path d="m21.5 2.5-7 19-4-8-8-4z" /></>, p);
export const IconChat = (p: P) => svg(<path d="M7.9 20A9 9 0 1 0 4 16.1L2.5 21.5z" />, p);
export const IconMenu = (p: P) => svg(<path d="M4 6h16M4 12h16M4 18h16" />, p);
export const IconX = (p: P) => svg(<path d="M18 6 6 18M6 6l12 12" />, p);
export const IconSun = (p: P) => svg(<><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></>, p);
export const IconMoon = (p: P) => svg(<path d="M20.5 13.2A8.5 8.5 0 1 1 10.8 3.5a6.6 6.6 0 0 0 9.7 9.7z" />, p);
export const IconChevronDown = (p: P) => svg(<path d="m6 9 6 6 6-6" />, p);
export const IconChevronLeft = (p: P) => svg(<path d="m15 18-6-6 6-6" />, p);
export const IconChevronRight = (p: P) => svg(<path d="m9 18 6-6-6-6" />, p);
export const IconExternal = (p: P) => svg(<><path d="M14 4h6v6" /><path d="M20 4 11 13" /><path d="M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5" /></>, p);
export const IconArrowRight = (p: P) => svg(<path d="M5 12h14M13 6l6 6-6 6" />, p);
export const IconFile = (p: P) => svg(<><path d="M14 2.5H6.5a2 2 0 0 0-2 2v15a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2V8z" /><path d="M14 2.5V8h5.5M8.5 13h7M8.5 17h5" /></>, p);
export const IconUpload = (p: P) => svg(<><path d="M12 15V3.5M7 8.5l5-5 5 5" /><path d="M20.5 15v4a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2v-4" /></>, p);
export const IconPaperclip = (p: P) => svg(<path d="m21 11-8.6 8.6a5 5 0 0 1-7-7l8.6-8.6a3.3 3.3 0 0 1 4.7 4.7l-8.6 8.6a1.7 1.7 0 0 1-2.4-2.4l8-8" />, p);
export const IconLock = (p: P) => svg(<><rect x="4" y="11" width="16" height="10" rx="2" /><path d="M8 11V7a4 4 0 0 1 8 0v4" /></>, p);
export const IconCheck = (p: P) => svg(<path d="M20 6 9 17l-5-5" />, p);
export const IconAlert = (p: P) => svg(<><path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z" /><path d="M12 9v4M12 17h.01" /></>, p);
export const IconPin = (p: P) => svg(<><path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0z" /><circle cx="12" cy="10" r="3" /></>, p);
export const IconPlus = (p: P) => svg(<path d="M12 5v14M5 12h14" />, p);
export const IconSearch = (p: P) => svg(<><circle cx="11" cy="11" r="7" /><path d="m20.5 20.5-4.2-4.2" /></>, p);
