import React from "react";
import type { Metadata } from "next";
import { SITE_CONFIG } from "@/config/site";

export const metadata: Metadata = {
  title: "Syarat & Ketentuan — Global Mandiri Computer",
};

export default function TermsPage() {
  return (
    <div className="py-16 md:py-24 bg-bg min-h-screen">
      <div className="gmc-container max-w-3xl">
        <p className="micro-label text-ink-3">KETENTUAN LAYANAN</p>
        <h1 className="heading-section text-ink mt-2 mb-8">
          Syarat &amp; Ketentuan
        </h1>

        <div className="space-y-6 text-sm text-ink-2 leading-relaxed border-t border-line pt-8">
          <p>
            Selamat datang di <strong>{SITE_CONFIG.name}</strong>. Dengan mengakses,
            menjelajahi katalog, atau melakukan pemesanan perangkat di situs ini,
            Anda setuju untuk terikat oleh Syarat dan Ketentuan berikut.
          </p>

          <h2 className="text-base font-medium text-ink pt-4">1. Informasi Produk &amp; Harga</h2>
          <p>
            Seluruh harga yang tertera disajikan dalam mata uang Rupiah (IDR) dan
            merupakan harga resmi yang berlaku pada saat pemesanan dibuat. Kami
            berusaha menjaga keakuratan stok dan harga; namun, bila terjadi kekeliruan
            sistem, tim kami berhak mengonfirmasi ulang atau membatalkan pesanan.
          </p>

          <h2 className="text-base font-medium text-ink pt-4">2. Garansi Resmi Distributor</h2>
          <p>
            Semua komponen PC, laptop, monitor, printer, dan perangkat Starlink
            yang kami jual adalah 100% original bergaransi distributor resmi di Indonesia.
            Klaim garansi tunduk pada syarat dan ketentuan yang ditetapkan oleh pemegang
            merek distributor masing-masing.
          </p>

          <h2 className="text-base font-medium text-ink pt-4">3. Pengambilan &amp; Pengiriman</h2>
          <p>
            Pesanan yang diambil langsung di showroom ITC Mega Grosir Surabaya
            wajib menunjukkan nomor pesanan atau bukti pembayaran yang sah.
            Untuk pengiriman kurir dan ekspedisi kargo, nomor resi pengiriman
            diberikan setelah paket diserahkan ke pihak ekspedisi.
          </p>
        </div>
      </div>
    </div>
  );
}
