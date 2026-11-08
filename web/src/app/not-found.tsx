import React from "react";
import Link from "next/link";
import { GmcWordmark } from "@/components/store/GmcWordmark";

export default function NotFound() {
  return (
    <div className="min-h-screen bg-bg flex flex-col items-center justify-center p-6 text-center">
      <Link href="/" className="mb-8">
        <GmcWordmark />
      </Link>
      <span className="micro-label text-ink-3 font-mono">KESALAHAN 404</span>
      <h1 className="heading-section text-ink mt-2 mb-4">
        Halaman Tidak Ditemukan.
      </h1>
      <p className="text-sm md:text-base text-ink-2 max-w-md mb-8 leading-relaxed">
        Halaman atau produk yang Anda tuju mungkin telah dipindahkan, berganti nama,
        atau tidak lagi tersedia di katalog kami.
      </p>
      <div className="flex flex-wrap items-center justify-center gap-4">
        <Link href="/" className="btn-pill-primary">
          Kembali ke Beranda
        </Link>
        <Link href="/shop" className="btn-pill-secondary">
          Buka Katalog Toko
        </Link>
      </div>
    </div>
  );
}
