"use client";

import React from "react";
import { ArrowUpRight, MapPin, Clock, MessageSquare } from "lucide-react";
import { SITE_CONFIG } from "@/config/site";

export function VisitUs() {
  return (
    <section className="py-20 md:py-32 bg-panel border-b border-line">
      <div className="gmc-container">
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 items-center">
          {/* Left: Info */}
          <div className="lg:col-span-7 space-y-8">
            <div>
              <p className="micro-label text-ink-3">KUNJUNGI SHOWROOM</p>
              <h2 className="heading-section text-ink mt-2">
                Datang langsung ke toko fisik kami di Surabaya.
              </h2>
            </div>

            <p className="text-base md:text-lg text-ink-2 max-w-xl leading-relaxed">
              Konsultasikan racikan PC rakitan Anda langsung bersama teknisi kami,
              cek fisik unit laptop sebelum transaksi, atau ambil pesanan yang telah
              Anda bayar secara online pada hari yang sama.
            </p>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-6 pt-4 border-t border-line">
              <div className="space-y-2">
                <div className="flex items-center gap-2 micro-label text-ink-3">
                  <MapPin className="w-4 h-4 text-ink" />
                  <span>ALAMAT SHOWROOM</span>
                </div>
                <p className="text-base text-ink font-medium">
                  {SITE_CONFIG.location.address}
                </p>
                <p className="text-sm text-ink-2">
                  Pusat perbelanjaan komputer ITC Mega Grosir Surabaya
                </p>
              </div>

              <div className="space-y-2">
                <div className="flex items-center gap-2 micro-label text-ink-3">
                  <Clock className="w-4 h-4 text-ink" />
                  <span>JAM OPERASIONAL</span>
                </div>
                <p className="text-base text-ink font-medium">
                  {SITE_CONFIG.openingHours}
                </p>
                <p className="text-sm text-ink-2">
                  Pengiriman kurir instan &amp; reguler setiap hari kerja
                </p>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-4 pt-4">
              <a
                href={SITE_CONFIG.location.mapsUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="btn-pill-primary inline-flex items-center gap-1.5"
              >
                <span>Buka di Google Maps</span>
                <ArrowUpRight className="w-4 h-4" />
              </a>

              <a
                href={SITE_CONFIG.contact.whatsappUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="btn-pill-secondary inline-flex items-center gap-1.5"
              >
                <MessageSquare className="w-4 h-4" />
                <span>Chat WhatsApp Admin</span>
              </a>
            </div>
          </div>

          {/* Right: Architectural Store Map / Graphic Card */}
          <div className="lg:col-span-5">
            <div className="aspect-square w-full bg-bg border border-line p-8 flex flex-col justify-between">
              <div className="flex justify-between items-start">
                <span className="micro-label text-ink-3">PANDUAN LOKASI</span>
                <span className="micro-label font-mono text-ink">LT. 2 / B-7</span>
              </div>

              <div className="my-auto text-center py-6">
                <div className="w-24 h-24 rounded-full border border-line mx-auto flex items-center justify-center mb-4">
                  <MapPin className="w-8 h-8 text-ink stroke-[1.5]" />
                </div>
                <h3 className="text-xl font-medium text-ink">ITC Mega Grosir</h3>
                <p className="text-sm text-ink-2 mt-1">Surabaya, Jawa Timur</p>
                <div className="mt-4 inline-block px-3 py-1 rounded-full bg-panel text-xs text-ink-2 border border-line">
                  Akses lift &amp; eskalator utama blok B
                </div>
              </div>

              <div className="pt-4 border-t border-line flex justify-between items-center text-xs text-ink-2">
                <span>Parkir Mobil &amp; Motor Luas</span>
                <span className="text-ink font-medium">Lantai 2 Blok B/7</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
