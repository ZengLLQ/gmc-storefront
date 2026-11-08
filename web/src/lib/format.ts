import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

const idrFormatter = new Intl.NumberFormat("id-ID", {
  maximumFractionDigits: 0,
  minimumFractionDigits: 0,
});

/**
 * Formats integer Rupiah amount as `Rp 1.234.567` with no decimals.
 */
export function formatIDR(amount: number | null | undefined): string {
  if (amount === null || amount === undefined || Number.isNaN(amount)) {
    return "Rp 0";
  }
  const rounded = Math.round(amount);
  if (rounded < 0) {
    return `-Rp ${idrFormatter.format(Math.abs(rounded))}`;
  }
  return `Rp ${idrFormatter.format(rounded)}`;
}

export function formatNumberID(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return "0";
  }
  return idrFormatter.format(Math.round(value));
}

export function formatDateID(iso: string | Date | null | undefined): string {
  if (!iso) return "-";
  const d = typeof iso === "string" ? new Date(iso) : iso;
  if (Number.isNaN(d.getTime())) return "-";
  return new Intl.DateTimeFormat("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    timeZone: "Asia/Jakarta",
  }).format(d);
}

export function formatDateTimeID(iso: string | Date | null | undefined): string {
  if (!iso) return "-";
  const d = typeof iso === "string" ? new Date(iso) : iso;
  if (Number.isNaN(d.getTime())) return "-";
  return (
    new Intl.DateTimeFormat("id-ID", {
      day: "2-digit",
      month: "short",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      timeZone: "Asia/Jakarta",
    }).format(d) + " WIB"
  );
}

/**
 * Normalizes Indonesian phone numbers (08xx, +628xx, 628xx) to 628xxxxxxxxxx.
 */
export function normalizePhoneID(raw: string): string {
  const digits = raw.replace(/[^0-9]/g, "");
  if (digits.startsWith("08")) {
    return "62" + digits.slice(1);
  }
  if (digits.startsWith("8")) {
    return "62" + digits;
  }
  return digits;
}

export function isValidPhoneID(raw: string): boolean {
  const norm = normalizePhoneID(raw);
  return /^628[0-9]{7,12}$/.test(norm);
}

/**
 * Generates a studio-style SVG placeholder data URI on `--panel` (#F2EFE8)
 * when a product or category image is missing or fails to load.
 */
export function makeHardwarePlaceholderSVG(
  brand = "GMC",
  title = "Perangkat Keras",
  variant: "primary" | "secondary" = "primary"
): string {
  const bg = variant === "primary" ? "#F2EFE8" : "#EAE5DA";
  const safeBrand = brand.replace(/[<>&"']/g, "").toUpperCase().slice(0, 22);
  const safeTitle = title.replace(/[<>&"']/g, "").slice(0, 36);
  const viewTag = variant === "primary" ? "STUDIO VIEW 01" : "DETAIL SPEC 02";

  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 800" width="800" height="800">
    <rect width="800" height="800" fill="${bg}"/>
    <text x="64" y="76" font-family="Inter, sans-serif" font-size="14" font-weight="500" letter-spacing="3" fill="#111111">${safeBrand}</text>
    <text x="736" y="76" text-anchor="end" font-family="Inter, sans-serif" font-size="12" font-weight="400" letter-spacing="2" fill="#8B877E">${viewTag}</text>
    <line x1="64" y1="98" x2="736" y2="98" stroke="#DAD6CC" stroke-width="1"/>
    <rect x="220" y="210" width="360" height="310" rx="6" fill="#1D1C1A" stroke="#111111" stroke-width="2"/>
    <rect x="248" y="238" width="304" height="254" rx="3" fill="#2A2825" stroke="#5F5C55" stroke-width="1"/>
    <circle cx="400" cy="365" r="52" fill="none" stroke="#8B877E" stroke-width="1.5"/>
    <line x1="64" y1="680" x2="736" y2="680" stroke="#DAD6CC" stroke-width="1"/>
    <text x="64" y="720" font-family="Inter, sans-serif" font-size="19" font-weight="500" fill="#111111">${safeTitle}</text>
    <text x="64" y="748" font-family="Inter, sans-serif" font-size="12" font-weight="400" letter-spacing="1.5" fill="#5F5C55">GLOBAL MANDIRI COMPUTER · SURABAYA</text>
  </svg>`;

  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

export const ORDER_STATUS_LABELS: Record<string, string> = {
  menunggu_konfirmasi: "Menunggu Konfirmasi",
  menunggu_pembayaran: "Menunggu Pembayaran",
  diproses: "Diproses",
  dikirim: "Dikirim",
  siap_diambil: "Siap Diambil",
  selesai: "Selesai",
  dibatalkan: "Dibatalkan",
};

export const PAYMENT_STATUS_LABELS: Record<string, string> = {
  unpaid: "Belum Dibayar",
  paid: "Lunas",
  refunded: "Dikembalikan",
};

export const DELIVERY_METHOD_LABELS: Record<string, string> = {
  pickup: "Ambil di Toko (ITC Mega Grosir Lt. 2 Blok B/7)",
  delivery: "Kirim ke Alamat",
};

export const PAYMENT_METHOD_LABELS: Record<string, string> = {
  bank_transfer: "Transfer Bank",
  cod: "COD / Konfirmasi via WhatsApp",
  pay_at_store: "Bayar saat Ambil di Toko",
};

export const STOCK_STATUS_LABELS: Record<string, string> = {
  in_stock: "Stok Tersedia",
  low_stock: "Stok Terbatas",
  out_of_stock: "Stok Habis",
  preorder: "Pre-order",
};
