import React from "react";
import type { Metadata } from "next";
import { SITE_CONFIG } from "@/config/site";

export const metadata: Metadata = {
  title: "Kebijakan Privasi — Global Mandiri Computer",
};

export default function PrivacyPage() {
  return (
    <div className="py-16 md:py-24 bg-bg min-h-screen">
      <div className="gmc-container max-w-3xl">
        <p className="micro-label text-ink-3">LEGAL &amp; PRIVASI</p>
        <h1 className="heading-section text-ink mt-2 mb-8">
          Kebijakan Privasi
        </h1>

        <div className="space-y-6 text-sm text-ink-2 leading-relaxed border-t border-line pt-8">
          <p>
            Di <strong>{SITE_CONFIG.name}</strong>, kami menghargai dan melindungi
            data pribadi setiap pelanggan kami. Kebijakan Privasi ini menjelaskan
            bagaimana data pribadi Anda dikumpulkan, digunakan, dan disimpan saat
            mengakses situs maupun berbelanja di showroom kami di ITC Mega Grosir Surabaya.
          </p>

          <h2 className="text-base font-medium text-ink pt-4">1. Data yang Kami Kumpulkan</h2>
          <p>
            Data yang kami perlukan untuk memproses transaksi mencakup nama lengkap,
            alamat pengiriman, alamat email, serta nomor WhatsApp yang aktif. Data ini
            digunakan khusus untuk keperluan pengiriman resi, invoice digital, serta
            koordinasi status pemesanan.
          </p>

          <h2 className="text-base font-medium text-ink pt-4">2. Keamanan Data</h2>
          <p>
            Kami menerapkan standar enkripsi data yang ketat. Informasi kartu kredit
            dan data transaksi perbankan diproses melalui gerbang pembayaran terenkripsi
            dan tidak disimpan langsung di peladen publik kami.
          </p>

          <h2 className="text-base font-medium text-ink pt-4">3. Kontak Kami</h2>
          <p>
            Jika Anda memiliki pertanyaan seputar Kebijakan Privasi ini, silakan hubungi
            kami melalui email di <u>{SITE_CONFIG.contact.email}</u> atau melalui WhatsApp
            resmi di <u>{SITE_CONFIG.contact.whatsappDisplay}</u>.
          </p>
        </div>
      </div>
    </div>
  );
}
