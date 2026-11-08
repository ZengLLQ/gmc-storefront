import React from "react";
import Link from "next/link";
import type { Metadata } from "next";
import { CheckCircle2, MessageSquare, ArrowRight, Store, CreditCard } from "lucide-react";
import { SITE_CONFIG } from "@/config/site";

export const metadata: Metadata = {
  title: "Pesanan Berhasil — Global Mandiri Computer",
};

export default async function OrderSuccessPage({
  params,
}: {
  params: Promise<{ orderNo: string }>;
}) {
  const { orderNo } = await params;

  const waText = encodeURIComponent(
    `Halo Admin GMC Surabaya, saya telah melakukan pemesanan dengan Nomor Pesanan: ${orderNo}. Mohon konfirmasi ketersediaan dan panduan pembayarannya. Terima kasih.`
  );
  const waConfirmUrl = `${SITE_CONFIG.contact.whatsappUrl}?text=${waText}`;

  return (
    <div className="py-16 md:py-28 bg-bg min-h-[75vh]">
      <div className="gmc-container max-w-2xl text-center">
        <div className="w-16 h-16 rounded-full bg-success/10 text-success mx-auto flex items-center justify-center mb-6">
          <CheckCircle2 className="w-10 h-10 stroke-[1.5]" />
        </div>

        <p className="micro-label text-ink-3">PESANAN TERKONFIRMASI</p>
        <h1 className="heading-section text-ink mt-2">Terima Kasih!</h1>
        <p className="text-base md:text-lg text-ink-2 mt-4 leading-relaxed">
          Pesanan Anda telah kami terima dan sedang disiapkan oleh tim Global Mandiri Computer.
        </p>

        {/* Order Number Box */}
        <div className="my-8 p-6 bg-panel border border-line text-center">
          <p className="micro-label text-ink-3 mb-1">NOMOR PESANAN ANDA</p>
          <p className="text-2xl md:text-3xl font-mono font-medium text-ink tracking-wider">
            {orderNo}
          </p>
          <p className="text-xs text-ink-2 mt-2">
            Simpan nomor ini untuk verifikasi atau pengambilan pesanan di toko fisik.
          </p>
        </div>

        {/* Next Steps / Payment Instructions */}
        <div className="text-left bg-bg border border-line p-6 space-y-4 mb-8">
          <h2 className="micro-label text-ink flex items-center gap-2">
            <CreditCard className="w-4 h-4" />
            <span>INSTRUKSI PEMBAYARAN &amp; VERIFIKASI</span>
          </h2>

          <div className="text-sm text-ink-2 space-y-2">
            <p>
              Silakan lakukan transfer ke salah satu rekening resmi kami:
            </p>
            <div className="p-3 bg-panel border border-line text-xs font-mono space-y-1 text-ink">
              <p><strong>BCA:</strong> 012-345-6789 (a.n. Global Mandiri Computer)</p>
              <p><strong>Mandiri:</strong> 142-00-1234567-8 (a.n. Global Mandiri Computer)</p>
            </div>
            <p className="text-xs text-ink-3 pt-1">
              Setelah transfer, kirimkan bukti pembayaran melalui WhatsApp untuk percepatan proses packing dan pengujian hardware.
            </p>
          </div>

          <div className="pt-3 border-t border-line flex items-start gap-2.5 text-xs text-ink-2">
            <Store className="w-4 h-4 text-ink shrink-0 mt-0.5" />
            <div>
              <p className="font-medium text-ink">Pengambilan di Toko Fisik</p>
              <p>
                {SITE_CONFIG.location.address}. Jam: {SITE_CONFIG.openingHours}.
              </p>
            </div>
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
          <a
            href={waConfirmUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="btn-pill-primary w-full sm:w-auto inline-flex items-center justify-center gap-2"
          >
            <MessageSquare className="w-4 h-4" />
            <span>Konfirmasi via WhatsApp Sekarang</span>
          </a>

          <Link
            href="/shop"
            className="btn-pill-secondary w-full sm:w-auto inline-flex items-center justify-center gap-2"
          >
            <span>Lanjut Belanja</span>
            <ArrowRight className="w-4 h-4" />
          </Link>
        </div>
      </div>
    </div>
  );
}
