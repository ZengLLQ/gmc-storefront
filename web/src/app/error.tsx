"use client";

import React, { useEffect } from "react";
import Link from "next/link";
import { GmcWordmark } from "@/components/store/GmcWordmark";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("App boundary error caught:", error);
  }, [error]);

  return (
    <div className="min-h-screen bg-bg flex flex-col items-center justify-center p-6 text-center">
      <Link href="/" className="mb-8">
        <GmcWordmark />
      </Link>
      <span className="micro-label text-danger font-mono">KESALAHAN SISTEM</span>
      <h1 className="heading-sub text-ink mt-2 mb-4">
        Terjadi Kendala Memuat Halaman.
      </h1>
      <p className="text-sm text-ink-2 max-w-md mb-8 leading-relaxed">
        Sistem mengalami gangguan teknis sementara. Silakan coba muat ulang halaman
        atau kembali ke beranda.
      </p>
      <div className="flex flex-wrap items-center justify-center gap-4">
        <button
          type="button"
          onClick={() => reset()}
          className="btn-pill-primary cursor-pointer"
        >
          Coba Lagi
        </button>
        <Link href="/" className="btn-pill-secondary">
          Ke Beranda
        </Link>
      </div>
    </div>
  );
}
