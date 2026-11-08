import { SITE_CONFIG } from "@/config/site";
import { formatIDR, normalizePhoneID } from "./format";

export interface WhatsAppOrderItemInput {
  name: string;
  variantLabel?: string | null;
  quantity: number;
  lineTotalIdr: number;
}

export interface WhatsAppOrderInput {
  orderNo: string;
  customerName: string;
  deliveryMethod: "pickup" | "delivery" | string;
  city?: string;
  items: WhatsAppOrderItemInput[];
  totalIdr: number;
  whatsappNumber?: string;
}

export function buildOrderWhatsAppUrl(order: WhatsAppOrderInput): string {
  const num = normalizePhoneID(
    order.whatsappNumber || SITE_CONFIG.contact.whatsappNumber
  );
  const methodText =
    order.deliveryMethod === "delivery"
      ? order.city
        ? `Kirim ke ${order.city}`
        : "Kirim ke alamat"
      : "Ambil di toko";

  const visibleItems = order.items.slice(0, 15);
  const extraCount = Math.max(0, order.items.length - 15);

  const lines = visibleItems.map((it, idx) => {
    const vLabel = it.variantLabel ? ` (${it.variantLabel})` : "";
    return `${idx + 1}. ${it.name}${vLabel} x${it.quantity} - ${formatIDR(
      it.lineTotalIdr
    )}`;
  });
  if (extraCount > 0) {
    lines.push(`+${extraCount} produk lainnya`);
  }

  const text = [
    `Halo ${SITE_CONFIG.name}, saya ingin konfirmasi pesanan:`,
    "",
    `No. Pesanan: ${order.orderNo}`,
    `Nama: ${order.customerName}`,
    `Metode: ${methodText}`,
    "",
    "Pesanan:",
    ...lines,
    `Total: ${formatIDR(order.totalIdr)}`,
    "",
    "Mohon info total ongkir & ketersediaan. Terima kasih.",
  ].join("\n");

  return `https://wa.me/${num}?text=${encodeURIComponent(text)}`;
}

export function buildProductWhatsAppUrl(
  productName: string,
  productSlug: string,
  variantLabel?: string | null,
  whatsappNumber?: string
): string {
  const num = normalizePhoneID(
    whatsappNumber || SITE_CONFIG.contact.whatsappNumber
  );
  const label = variantLabel ? ` (${variantLabel})` : "";
  const url =
    typeof window !== "undefined"
      ? `${window.location.origin}/shop/${productSlug}`
      : `/shop/${productSlug}`;
  const text = `Halo, saya tertarik dengan ${productName}${label} (${url}). Apakah stok tersedia di ITC Mega Grosir?`;
  return `https://wa.me/${num}?text=${encodeURIComponent(text)}`;
}

export function buildContactWhatsAppUrl(params: {
  name: string;
  subject: string;
  message: string;
  whatsappNumber?: string;
}): string {
  const num = normalizePhoneID(
    params.whatsappNumber || SITE_CONFIG.contact.whatsappNumber
  );
  const text = `Halo ${SITE_CONFIG.name}, saya ${params.name} ingin bertanya perihal [${params.subject}]:\n${params.message}`;
  return `https://wa.me/${num}?text=${encodeURIComponent(text)}`;
}

export function buildGeneralWhatsAppUrl(
  customMessage = `Halo ${SITE_CONFIG.name}, saya ingin berkonsultasi mengenai produk komputer di GMC.`,
  whatsappNumber?: string
): string {
  const num = normalizePhoneID(
    whatsappNumber || SITE_CONFIG.contact.whatsappNumber
  );
  return `https://wa.me/${num}?text=${encodeURIComponent(customMessage)}`;
}
