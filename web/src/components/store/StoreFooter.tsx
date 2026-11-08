"use client";

import React, { useState } from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { SITE_CONFIG } from "@/config/site";
import { SITE_COPY } from "@/config/copy";
import { GmcWordmark } from "./GmcWordmark";
import { apiPost } from "@/lib/api/client";

export function StoreFooter() {
  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [firstName, setFirstName] = useState("");
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const handleNewsletterSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMsg(null);

    if (step === 1) {
      if (!firstName.trim()) {
        setErrorMsg("Silakan masukkan nama depan Anda.");
        return;
      }
      setStep(2);
      return;
    }

    if (step === 2) {
      if (!email.trim() || !email.includes("@")) {
        setErrorMsg("Silakan masukkan alamat email yang valid.");
        return;
      }
      setSubmitting(true);
      try {
        await apiPost("/v1/newsletter/subscribe", {
          name: firstName.trim(),
          email: email.trim(),
        });
        setStep(3);
      } catch (err) {
        setErrorMsg(
          err instanceof Error ? err.message : "Gagal mendaftarkan email."
        );
      } finally {
        setSubmitting(false);
      }
    }
  };

  return (
    <footer className="bg-footer border-t border-line text-ink pt-16 md:pt-24 pb-10">
      <div className="gmc-container">
        {/* Top Row: Left 2-step Huge Newsletter Input + Right 3 Link Columns */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 lg:gap-8 pb-16 border-b border-line">
          {/* Left 6 Cols: Newsletter */}
          <div className="lg:col-span-6 lg:pr-12">
            <p className="text-sm text-ink-2 mb-5">
              {SITE_COPY.footer.newsletterLabel}
            </p>

            {step === 3 ? (
              <div className="py-4 border-b border-line">
                <p className="text-2xl md:text-3xl font-normal tracking-tight text-ink">
                  Terima kasih telah bergabung, {firstName}.
                </p>
                <p className="text-sm text-ink-2 mt-2">
                  Kami akan mengirimkan kabar produk pilihan dan pembaruan stok
                  GMC ke {email}.
                </p>
              </div>
            ) : (
              <form onSubmit={handleNewsletterSubmit} className="relative">
                <div className="flex items-center border-b border-ink pb-3">
                  {step === 1 ? (
                    <input
                      type="text"
                      value={firstName}
                      onChange={(e) => setFirstName(e.target.value)}
                      placeholder={SITE_COPY.footer.newsletterStep1Placeholder}
                      aria-label="Nama depan Anda"
                      className="w-full bg-transparent text-[clamp(26px,3.6vw,44px)] font-normal tracking-tight text-ink placeholder:text-ink-3 focus:outline-none leading-tight"
                    />
                  ) : (
                    <input
                      type="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder={SITE_COPY.footer.newsletterStep2Placeholder}
                      aria-label="Alamat email Anda"
                      autoFocus
                      className="w-full bg-transparent text-[clamp(26px,3.6vw,44px)] font-normal tracking-tight text-ink placeholder:text-ink-3 focus:outline-none leading-tight"
                    />
                  )}
                  <button
                    type="submit"
                    disabled={submitting}
                    aria-label="Lanjutkan pendaftaran newsletter"
                    className="ml-4 p-2 text-ink hover:opacity-70 transition-opacity cursor-pointer shrink-0"
                  >
                    <ArrowRight className="w-7 h-7 stroke-[1.5]" />
                  </button>
                </div>

                <div className="flex items-center justify-between mt-2.5">
                  <span className="micro-label text-ink-3">
                    {step === 1 ? "LANGKAH 01 / 02 — NAMA" : "LANGKAH 02 / 02 — EMAIL"}
                  </span>
                  {step === 2 && (
                    <button
                      type="button"
                      onClick={() => setStep(1)}
                      className="micro-label text-ink-2 hover:text-ink underline cursor-pointer"
                    >
                      Ubah nama
                    </button>
                  )}
                </div>
                {errorMsg && (
                  <p className="text-xs text-danger mt-2">{errorMsg}</p>
                )}
              </form>
            )}
          </div>

          {/* Right 6 Cols: 3 Link Columns */}
          <div className="lg:col-span-6 grid grid-cols-2 sm:grid-cols-3 gap-8">
            <div>
              <h3 className="micro-label text-ink-3 mb-4">PRODUK</h3>
              <ul className="space-y-2.5 text-[15px] text-ink-2">
                <li>
                  <Link
                    href="/shop?category=komputer-rakitan"
                    className="hover:text-ink transition-colors"
                  >
                    PC &amp; Komponen
                  </Link>
                </li>
                <li>
                  <Link
                    href="/shop?category=laptop"
                    className="hover:text-ink transition-colors"
                  >
                    Laptop
                  </Link>
                </li>
                <li>
                  <Link
                    href="/shop?category=monitor"
                    className="hover:text-ink transition-colors"
                  >
                    Monitor &amp; Proyektor
                  </Link>
                </li>
                <li>
                  <Link
                    href="/shop?category=starlink"
                    className="hover:text-ink transition-colors"
                  >
                    Networking &amp; Starlink
                  </Link>
                </li>
                <li>
                  <Link
                    href="/shop?category=printer"
                    className="hover:text-ink transition-colors"
                  >
                    Printer &amp; Aksesoris
                  </Link>
                </li>
                <li>
                  <Link
                    href="/shop"
                    className="hover:text-ink transition-colors"
                  >
                    Lihat Semua
                  </Link>
                </li>
              </ul>
            </div>

            <div>
              <h3 className="micro-label text-ink-3 mb-4">INFORMASI</h3>
              <ul className="space-y-2.5 text-[15px] text-ink-2">
                <li>
                  <Link
                    href="/about"
                    className="hover:text-ink transition-colors"
                  >
                    Tentang GMC
                  </Link>
                </li>
                <li>
                  <Link
                    href="/contact"
                    className="hover:text-ink transition-colors"
                  >
                    Kontak Kami
                  </Link>
                </li>
                <li>
                  <Link
                    href="/contact#pengiriman"
                    className="hover:text-ink transition-colors"
                  >
                    Pengiriman
                  </Link>
                </li>
                <li>
                  <Link
                    href="/contact#garansi"
                    className="hover:text-ink transition-colors"
                  >
                    Garansi Resmi
                  </Link>
                </li>
                <li>
                  <Link
                    href="/account"
                    className="hover:text-ink transition-colors"
                  >
                    Akun Saya
                  </Link>
                </li>
              </ul>
            </div>

            <div>
              <h3 className="micro-label text-ink-3 mb-4">BANTUAN</h3>
              <ul className="space-y-2.5 text-[15px] text-ink-2">
                <li>
                  <Link
                    href="/contact#faq"
                    className="hover:text-ink transition-colors"
                  >
                    FAQ
                  </Link>
                </li>
                <li>
                  <Link
                    href="/contact#pembayaran"
                    className="hover:text-ink transition-colors"
                  >
                    Cara Pesan
                  </Link>
                </li>
                <li>
                  <Link
                    href="/contact#pengembalian"
                    className="hover:text-ink transition-colors"
                  >
                    Pengembalian
                  </Link>
                </li>
                <li>
                  <Link
                    href="/privacy"
                    className="hover:text-ink transition-colors"
                  >
                    Kebijakan Privasi
                  </Link>
                </li>
                <li>
                  <Link
                    href="/terms"
                    className="hover:text-ink transition-colors"
                  >
                    Syarat &amp; Ketentuan
                  </Link>
                </li>
              </ul>
            </div>
          </div>
        </div>

        {/* Middle Row: 3 Stacked Hairline Information Blocks matching reference */}
        <div className="grid grid-cols-1 md:grid-cols-3 divide-y md:divide-y-0 md:divide-x divide-line border-b border-line">
          <div className="py-8 md:pr-8">
            <p className="micro-label text-ink-3 mb-3">HUBUNGI KAMI</p>
            <div className="space-y-1.5 text-sm text-ink">
              <p>
                WhatsApp:{" "}
                <a
                  href={SITE_CONFIG.contact.whatsappUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="underline hover:opacity-75"
                >
                  {SITE_CONFIG.contact.whatsappDisplay}
                </a>
              </p>
              <p>
                Email:{" "}
                <a
                  href={`mailto:${SITE_CONFIG.contact.email}`}
                  className="underline hover:opacity-75"
                >
                  {SITE_CONFIG.contact.email}
                </a>
              </p>
              <p className="text-ink-2">{SITE_CONFIG.openingHours}</p>
            </div>
          </div>

          <div className="py-8 md:px-8">
            <p className="micro-label text-ink-3 mb-3">LOKASI TOKO</p>
            <p className="text-sm text-ink mb-2">
              {SITE_CONFIG.location.address}
            </p>
            <a
              href={SITE_CONFIG.location.mapsUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="micro-label text-ink underline hover:opacity-75 inline-flex items-center gap-1"
            >
              Lihat di Google Maps →
            </a>
          </div>

          <div className="py-8 md:pl-8">
            <p className="micro-label text-ink-3 mb-3">METODE PEMBAYARAN</p>
            <div className="flex flex-wrap gap-2">
              {[
                "Transfer Bank",
                "QRIS",
                "Tunai di Toko",
                "Konfirmasi via WhatsApp",
              ].map((m) => (
                <span
                  key={m}
                  className="px-3 py-1 rounded-full border border-line text-xs text-ink-2"
                >
                  {m}
                </span>
              ))}
            </div>
          </div>
        </div>

        {/* Giant Full-Bleed GMC Wordmark */}
        <div className="pt-10 pb-6">
          <GmcWordmark size="hero-footer" />
        </div>

        {/* Bottom Copyright Bar */}
        <div className="pt-6 border-t border-line flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 text-xs text-ink-2">
          <p>
            © {new Date().getFullYear()} {SITE_CONFIG.name}. Semua hak
            dilindungi.
          </p>
          <div className="flex items-center gap-6">
            <Link href="/privacy" className="hover:text-ink transition-colors">
              Kebijakan Privasi
            </Link>
            <Link href="/terms" className="hover:text-ink transition-colors">
              Syarat &amp; Ketentuan
            </Link>
            <Link href="/account" className="hover:text-ink transition-colors">
              Akun Pelanggan
            </Link>
          </div>
        </div>
      </div>
    </footer>
  );
}
